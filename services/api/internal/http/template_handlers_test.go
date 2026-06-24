package http

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/templates"

	_ "modernc.org/sqlite"
)

func TestAdminTemplateFlowPublishesTemplateForPublicRoutes(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})

	createRecorder := doMultipartTemplateCreate(t, router, adminCookie, "Support Agent", "answers customers", "Original soul.", "Original user.", nil)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	created := decodeTemplateResponse(t, createRecorder.Body.Bytes()).Template

	addSkillRecorder := doMultipartUpload(t, router, "/api/admin/templates/"+created.ID+"/skills", adminCookie, map[string]string{
		"faq/SKILL.md": "---\nname: faq\ndescription: Frequently asked questions\n---\n# FAQ\n",
	})
	if addSkillRecorder.Code != http.StatusCreated {
		t.Fatalf("add skill status = %d, body = %s", addSkillRecorder.Code, addSkillRecorder.Body.String())
	}

	publishRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+created.ID+"/publication", `{}`, adminCookie)
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	published := decodeTemplateResponse(t, publishRecorder.Body.Bytes()).Template
	if published.Status != templates.StatusPublished {
		t.Fatalf("published template = %#v", published)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/templates", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("public list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	assertNoPathFields(t, listRecorder.Body.Bytes())
	var listResponse struct {
		Templates []templates.Template `json:"templates"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listResponse.Templates) != 1 || listResponse.Templates[0].ID != created.ID {
		t.Fatalf("public list = %#v", listResponse.Templates)
	}

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/api/templates/"+created.ID, nil))
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("public detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	assertNoPathFields(t, detailRecorder.Body.Bytes())
}

func TestAdminCanListDraftTemplatesWithoutPathFields(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	listRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/templates", nil)
	request.AddCookie(adminCookie)
	router.ServeHTTP(listRecorder, request)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	assertNoPathFields(t, listRecorder.Body.Bytes())
	if !bytes.Contains(listRecorder.Body.Bytes(), []byte(templateID)) {
		t.Fatalf("admin list body %s does not include draft template %s", listRecorder.Body.String(), templateID)
	}
}

func TestAdminListExcludesArchivedTemplates(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	archiveRecorder := httptest.NewRecorder()
	archiveRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID, nil)
	archiveRequest.AddCookie(adminCookie)
	router.ServeHTTP(archiveRecorder, archiveRequest)
	if archiveRecorder.Code != http.StatusNoContent {
		t.Fatalf("archive status = %d, body = %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates", nil)
	listRequest.AddCookie(adminCookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if bytes.Contains(listRecorder.Body.Bytes(), []byte(templateID)) {
		t.Fatalf("archived template leaked into admin list: %s", listRecorder.Body.String())
	}
}

func TestAdminCanCreateTemplateWithMultipartContentsAndSkillArchive(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	archive := makeSkillArchive(t, map[string]string{
		"SKILL.md": "---\nname: FAQ\ndescription: Frequently asked questions\n---\n# FAQ\n",
	})

	createRecorder := doMultipartTemplateCreate(t, router, adminCookie, "Support Agent", "answers customers", "Original soul.", "Original user.", []multipartSkillFile{
		{name: "faq.zip", content: archive},
	})
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	response := decodeTemplateResponse(t, createRecorder.Body.Bytes())

	soulRecorder := httptest.NewRecorder()
	soulRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+response.Template.ID+"/soul", nil)
	soulRequest.AddCookie(adminCookie)
	router.ServeHTTP(soulRecorder, soulRequest)
	if soulRecorder.Code != http.StatusOK || !bytes.Contains(soulRecorder.Body.Bytes(), []byte("Original soul.")) {
		t.Fatalf("soul status/body = %d / %s", soulRecorder.Code, soulRecorder.Body.String())
	}

	skillsRecorder := httptest.NewRecorder()
	skillsRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+response.Template.ID+"/skills", nil)
	skillsRequest.AddCookie(adminCookie)
	router.ServeHTTP(skillsRecorder, skillsRequest)
	if skillsRecorder.Code != http.StatusOK || !bytes.Contains(skillsRecorder.Body.Bytes(), []byte(`"skillName":"faq"`)) {
		t.Fatalf("skills status/body = %d / %s", skillsRecorder.Code, skillsRecorder.Body.String())
	}
}

func TestAdminCreateTemplateRejectsInvalidSkillArchive(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	archive := makeSkillArchive(t, map[string]string{
		"notes.md": "missing skill",
	})

	createRecorder := doMultipartTemplateCreate(t, router, adminCookie, "Broken Agent", "broken", "Original soul.", "Original user.", []multipartSkillFile{
		{name: "broken.zip", content: archive},
	})
	if createRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates", nil)
	listRequest.AddCookie(adminCookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	if bytes.Contains(listRecorder.Body.Bytes(), []byte("Broken Agent")) {
		t.Fatalf("broken template leaked into list: %s", listRecorder.Body.String())
	}
}

func TestAdminCanGetTemplateDetailWithoutPathFields(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+templateID, nil)
	detailRequest.AddCookie(adminCookie)
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("admin detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	assertNoPathFields(t, detailRecorder.Body.Bytes())
	if !bytes.Contains(detailRecorder.Body.Bytes(), []byte(templateID)) {
		t.Fatalf("admin detail body %s does not include template %s", detailRecorder.Body.String(), templateID)
	}
}

func TestPublicTemplateDetailHidesDraftAndArchivedTemplates(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	draftRecorder := httptest.NewRecorder()
	router.ServeHTTP(draftRecorder, httptest.NewRequest(http.MethodGet, "/api/templates/"+templateID, nil))
	if draftRecorder.Code != http.StatusNotFound {
		t.Fatalf("draft public detail status = %d, want 404", draftRecorder.Code)
	}

	if publishRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie); publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	archiveRecorder := httptest.NewRecorder()
	archiveRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID, nil)
	archiveRequest.AddCookie(adminCookie)
	router.ServeHTTP(archiveRecorder, archiveRequest)
	if archiveRecorder.Code != http.StatusNoContent {
		t.Fatalf("archive status = %d, body = %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}
	archivedRecorder := httptest.NewRecorder()
	router.ServeHTTP(archivedRecorder, httptest.NewRequest(http.MethodGet, "/api/templates/"+templateID, nil))
	if archivedRecorder.Code != http.StatusNotFound {
		t.Fatalf("archived public detail status = %d, want 404", archivedRecorder.Code)
	}
}

func TestDeletePublicationReturnsTemplateToDraft(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	if publishRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie); publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}

	unpublishRecorder := httptest.NewRecorder()
	unpublishRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID+"/publication", nil)
	unpublishRequest.AddCookie(adminCookie)
	router.ServeHTTP(unpublishRecorder, unpublishRequest)
	if unpublishRecorder.Code != http.StatusOK {
		t.Fatalf("unpublish status = %d, body = %s", unpublishRecorder.Code, unpublishRecorder.Body.String())
	}
	response := decodeTemplateResponse(t, unpublishRecorder.Body.Bytes()).Template
	if response.ID == templateID || response.Status != templates.StatusDraft || response.Version != 2 || response.PublishedAt != nil {
		t.Fatalf("unpublished template = %#v", response)
	}

	publicRecorder := httptest.NewRecorder()
	router.ServeHTTP(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/templates/"+templateID, nil))
	if publicRecorder.Code != http.StatusNotFound {
		t.Fatalf("public detail after unpublish status = %d, want 404", publicRecorder.Code)
	}

	originalRecorder := httptest.NewRecorder()
	originalRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+templateID+"/soul", nil)
	originalRequest.AddCookie(adminCookie)
	router.ServeHTTP(originalRecorder, originalRequest)
	if originalRecorder.Code != http.StatusOK || !bytes.Contains(originalRecorder.Body.Bytes(), []byte("Original soul.")) {
		t.Fatalf("original template after unpublish status = %d, body = %s", originalRecorder.Code, originalRecorder.Body.String())
	}
}

func TestDeletePublicationRejectsNonPublishedTemplate(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	draftRecorder := httptest.NewRecorder()
	draftRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID+"/publication", nil)
	draftRequest.AddCookie(adminCookie)
	router.ServeHTTP(draftRecorder, draftRequest)
	if draftRecorder.Code != http.StatusBadRequest {
		t.Fatalf("draft unpublish status = %d, body = %s", draftRecorder.Code, draftRecorder.Body.String())
	}

	archiveRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID, nil)
	archiveRequest.AddCookie(adminCookie)
	archiveRecorder := httptest.NewRecorder()
	router.ServeHTTP(archiveRecorder, archiveRequest)
	if archiveRecorder.Code != http.StatusNoContent {
		t.Fatalf("archive status = %d, body = %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}

	archivedRecorder := httptest.NewRecorder()
	archivedRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID+"/publication", nil)
	archivedRequest.AddCookie(adminCookie)
	router.ServeHTTP(archivedRecorder, archivedRequest)
	if archivedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("archived unpublish status = %d, body = %s", archivedRecorder.Code, archivedRecorder.Body.String())
	}
}

func TestAdminTemplateRoutesRequireAdmin(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	userCookie := sessionCookieFor(t, manager, auth.User{ID: "user-1", Email: "user@example.com", Role: auth.RoleUser})

	missingSession := doJSON(t, router, http.MethodPost, "/api/admin/templates", `{"name":"Support Agent"}`, nil)
	if missingSession.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status = %d, want 401", missingSession.Code)
	}
	nonAdmin := doJSON(t, router, http.MethodPost, "/api/admin/templates", `{"name":"Support Agent"}`, userCookie)
	if nonAdmin.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want 403", nonAdmin.Code)
	}
}

func TestAdminSkillRoutesRejectDuplicateAndDoNotExposeEditRoutes(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	first := doMultipartUpload(t, router, "/api/admin/templates/"+templateID+"/skills", adminCookie, map[string]string{
		"faq/SKILL.md": "---\nname: faq\ndescription: Frequently asked questions\n---\n# FAQ\n",
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("first skill status = %d, body = %s", first.Code, first.Body.String())
	}
	assertNoPathFields(t, first.Body.Bytes())
	skill := decodeSkillResponse(t, first.Body.Bytes()).Skill
	duplicate := doMultipartUpload(t, router, "/api/admin/templates/"+templateID+"/skills", adminCookie, map[string]string{
		"faq/SKILL.md": "---\nname: faq\ndescription: Frequently asked questions\n---\n# duplicate\n",
	})
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409, body = %s", duplicate.Code, duplicate.Body.String())
	}

	getSkill := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+templateID+"/skills/"+skill.ID, nil)
	request.AddCookie(adminCookie)
	router.ServeHTTP(getSkill, request)
	if getSkill.Code != http.StatusOK {
		t.Fatalf("get skill status = %d, body = %s", getSkill.Code, getSkill.Body.String())
	}
	assertNoPathFields(t, getSkill.Body.Bytes())

	editSkill := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/skills/"+skill.ID, `{"skillMD":"# edited\n"}`, adminCookie)
	if editSkill.Code != http.StatusMethodNotAllowed && editSkill.Code != http.StatusNotFound {
		t.Fatalf("skill edit status = %d, want 404 or 405", editSkill.Code)
	}

	deleteSkill := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID+"/skills/"+skill.ID, nil)
	deleteRequest.AddCookie(adminCookie)
	router.ServeHTTP(deleteSkill, deleteRequest)
	if deleteSkill.Code != http.StatusNoContent {
		t.Fatalf("delete skill status = %d, body = %s", deleteSkill.Code, deleteSkill.Body.String())
	}
	getDeleted := httptest.NewRecorder()
	getDeletedRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+templateID+"/skills/"+skill.ID, nil)
	getDeletedRequest.AddCookie(adminCookie)
	router.ServeHTTP(getDeleted, getDeletedRequest)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("get deleted skill status = %d, want 404", getDeleted.Code)
	}
}

func TestAdminSkillUploadRejectsInvalidArchives(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	cases := []struct {
		name   string
		files  map[string]string
		status int
	}{
		{
			name: "missing skill md",
			files: map[string]string{
				"faq/readme.md": "missing skill",
			},
			status: http.StatusBadRequest,
		},
		{
			name: "multiple top level directories",
			files: map[string]string{
				"faq/SKILL.md": "---\nname: faq\ndescription: Frequently asked questions\n---\n# FAQ\n",
				"ops/SKILL.md": "---\nname: ops\ndescription: Operations guide\n---\n# OPS\n",
			},
			status: http.StatusBadRequest,
		},
		{
			name: "path traversal",
			files: map[string]string{
				"faq/../oops.txt": "escape",
				"faq/SKILL.md":    "# FAQ\n",
			},
			status: http.StatusBadRequest,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := doMultipartUpload(t, router, "/api/admin/templates/"+templateID+"/skills", adminCookie, tt.files)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAddSkillArchiveRequiresMultipartFile(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/templates/"+templateID+"/skills", nil)
	request.AddCookie(adminCookie)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing multipart status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestEditingPublishedTemplateReturnsNewDraft(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)

	publishRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie)
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}

	editRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/soul", `{"content":"Changed soul."}`, adminCookie)
	if editRecorder.Code != http.StatusOK {
		t.Fatalf("edit published soul status = %d, body = %s", editRecorder.Code, editRecorder.Body.String())
	}
	draft := decodeTemplateResponse(t, editRecorder.Body.Bytes()).Template
	if draft.ID == templateID || draft.Status != templates.StatusDraft || draft.Version != 2 {
		t.Fatalf("draft after editing published = %#v, original id = %s", draft, templateID)
	}

	originalSoul := httptest.NewRecorder()
	originalSoulRequest := httptest.NewRequest(http.MethodGet, "/api/admin/templates/"+templateID+"/soul", nil)
	originalSoulRequest.AddCookie(adminCookie)
	router.ServeHTTP(originalSoul, originalSoulRequest)
	if originalSoul.Code != http.StatusOK || !bytes.Contains(originalSoul.Body.Bytes(), []byte("Original soul.")) {
		t.Fatalf("original soul response status = %d, body = %s", originalSoul.Code, originalSoul.Body.String())
	}
}

func TestDeletingPublishedSkillReturnsNewDraft(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})
	templateID := createCompleteDraftViaHTTP(t, router, adminCookie)
	addSkillRecorder := doMultipartUpload(t, router, "/api/admin/templates/"+templateID+"/skills", adminCookie, map[string]string{
		"faq/SKILL.md": "---\nname: faq\ndescription: Frequently asked questions\n---\n# FAQ\n",
	})
	if addSkillRecorder.Code != http.StatusCreated {
		t.Fatalf("add skill status = %d, body = %s", addSkillRecorder.Code, addSkillRecorder.Body.String())
	}
	skill := decodeSkillResponse(t, addSkillRecorder.Body.Bytes()).Skill
	if publishRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/publication", `{}`, adminCookie); publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/templates/"+templateID+"/skills/"+skill.ID, nil)
	deleteRequest.AddCookie(adminCookie)
	router.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete published skill status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	response := decodeTemplateResponse(t, deleteRecorder.Body.Bytes())
	if response.Template.ID == templateID || response.Template.Version != 2 || response.Template.Status != templates.StatusDraft {
		t.Fatalf("delete published skill response = %#v, original id = %s", response.Template, templateID)
	}
	assertNoPathFields(t, deleteRecorder.Body.Bytes())
}

func createCompleteDraftViaHTTP(t *testing.T, router http.Handler, adminCookie *http.Cookie) string {
	t.Helper()
	createRecorder := doMultipartTemplateCreate(t, router, adminCookie, "Support Agent", "", "Original soul.", "Original user.", nil)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	templateID := decodeTemplateResponse(t, createRecorder.Body.Bytes()).Template.ID
	return templateID
}

func doMultipartUpload(t *testing.T, handler http.Handler, path string, cookie *http.Cookie, files map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	archive := &bytes.Buffer{}
	zipWriter := zip.NewWriter(archive)
	for name, content := range files {
		entry, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	body := &bytes.Buffer{}
	formWriter := multipart.NewWriter(body)
	part, err := formWriter.CreateFormFile("file", "skill.zip")
	if err != nil {
		t.Fatalf("create multipart file: %v", err)
	}
	if _, err := part.Write(archive.Bytes()); err != nil {
		t.Fatalf("write multipart body: %v", err)
	}
	if err := formWriter.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body.Bytes()))
	request.Header.Set("content-type", formWriter.FormDataContentType())
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func newTemplateTestRouter(t *testing.T) (http.Handler, *auth.SessionManager) {
	t.Helper()
	database := newTemplateHTTPTestDB(t)
	manager := auth.NewSessionManager("test-secret", false)
	router := NewRouter(Dependencies{
		AuthRepository:  auth.NewRepository(database),
		SessionManager:  manager,
		TemplateService: templates.NewService(templates.NewRepository(database), templates.NewFileStore(t.TempDir())),
	})
	return router, manager
}

func newTemplateHTTPTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", "file:template-http-test-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE agent_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
			version INTEGER NOT NULL DEFAULT 1,
			template_path TEXT NOT NULL,
			content_checksum TEXT NOT NULL,
			soul_md_path TEXT NOT NULL DEFAULT '',
			user_md_path TEXT NOT NULL DEFAULT '',
			soul_content TEXT NOT NULL DEFAULT '',
			user_content TEXT NOT NULL DEFAULT '',
			skills_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			published_at TEXT,
			FOREIGN KEY (created_by) REFERENCES users(id)
		);
		CREATE TABLE template_skills (
			id TEXT PRIMARY KEY,
			template_id TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			skill_path TEXT NOT NULL,
			checksum TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (template_id) REFERENCES agent_templates(id) ON DELETE CASCADE,
			UNIQUE (template_id, skill_name)
		);
		INSERT INTO users (id, email, password_hash, role)
		VALUES ('admin-1', 'admin@example.com', 'unused', 'admin'),
		       ('user-1', 'user@example.com', 'unused', 'user');
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}
	return database
}

func sessionCookieFor(t *testing.T, manager *auth.SessionManager, user auth.User) *http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	if err := manager.SetSessionCookie(recorder, user); err != nil {
		t.Fatalf("SetSessionCookie returned error: %v", err)
	}
	return recorder.Result().Cookies()[0]
}

func doJSON(t *testing.T, router http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeTemplateResponse(t *testing.T, body []byte) struct {
	Template templates.Template `json:"template"`
} {
	t.Helper()
	var response struct {
		Template templates.Template `json:"template"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal template response %q: %v", body, err)
	}
	return response
}

func decodeSkillResponse(t *testing.T, body []byte) struct {
	Skill templates.Skill `json:"skill"`
} {
	t.Helper()
	var response struct {
		Skill templates.Skill `json:"skill"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("unmarshal skill response %q: %v", body, err)
	}
	return response
}

func assertNoPathFields(t *testing.T, body []byte) {
	t.Helper()
	for _, forbidden := range []string{"templatePath", "soulMDPath", "userMDPath", "skillsPath", "skillPath", "/templates/"} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}
}

type multipartSkillFile struct {
	name    string
	content []byte
}

func doMultipartTemplateCreate(t *testing.T, router http.Handler, cookie *http.Cookie, name, description, soulContent, userContent string, skills []multipartSkillFile) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for field, value := range map[string]string{
		"name":        name,
		"description": description,
		"soulContent": soulContent,
		"userContent": userContent,
	} {
		if err := writer.WriteField(field, value); err != nil {
			t.Fatalf("WriteField(%s): %v", field, err)
		}
	}
	for _, skill := range skills {
		part, err := writer.CreateFormFile("skillZips", skill.name)
		if err != nil {
			t.Fatalf("CreateFormFile(%s): %v", skill.name, err)
		}
		if _, err := part.Write(skill.content); err != nil {
			t.Fatalf("Write skill file %s: %v", skill.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/templates", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func makeSkillArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "skill.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s: %v", name, err)
		}
		if _, err := io.Copy(entry, bytes.NewBufferString(content)); err != nil {
			t.Fatalf("Write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close archive file: %v", err)
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Read archive: %v", err)
	}
	return data
}
