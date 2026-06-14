package http

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentforge.local/services/api/internal/auth"
	"agentforge.local/services/api/internal/templates"

	_ "modernc.org/sqlite"
)

func TestAdminTemplateFlowPublishesTemplateForPublicRoutes(t *testing.T) {
	router, manager := newTemplateTestRouter(t)
	adminCookie := sessionCookieFor(t, manager, auth.User{ID: "admin-1", Email: "admin@example.com", Role: auth.RoleAdmin})

	createRecorder := doJSON(t, router, http.MethodPost, "/api/admin/templates", `{"name":"Support Agent","description":"answers customers"}`, adminCookie)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	created := decodeTemplateResponse(t, createRecorder.Body.Bytes()).Template

	putSoulRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+created.ID+"/soul", `{"content":"Original soul."}`, adminCookie)
	if putSoulRecorder.Code != http.StatusOK {
		t.Fatalf("put soul status = %d, body = %s", putSoulRecorder.Code, putSoulRecorder.Body.String())
	}
	putUserRecorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+created.ID+"/user", `{"content":"Original user."}`, adminCookie)
	if putUserRecorder.Code != http.StatusOK {
		t.Fatalf("put user status = %d, body = %s", putUserRecorder.Code, putUserRecorder.Body.String())
	}
	addSkillRecorder := doJSON(t, router, http.MethodPost, "/api/admin/templates/"+created.ID+"/skills", `{"skillName":"faq","skillMD":"# FAQ\n"}`, adminCookie)
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

	first := doJSON(t, router, http.MethodPost, "/api/admin/templates/"+templateID+"/skills", `{"skillName":"faq","skillMD":"# FAQ\n"}`, adminCookie)
	if first.Code != http.StatusCreated {
		t.Fatalf("first skill status = %d, body = %s", first.Code, first.Body.String())
	}
	skill := decodeSkillResponse(t, first.Body.Bytes()).Skill
	duplicate := doJSON(t, router, http.MethodPost, "/api/admin/templates/"+templateID+"/skills", `{"skillName":"faq","skillMD":"# duplicate\n"}`, adminCookie)
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

func createCompleteDraftViaHTTP(t *testing.T, router http.Handler, adminCookie *http.Cookie) string {
	t.Helper()
	createRecorder := doJSON(t, router, http.MethodPost, "/api/admin/templates", `{"name":"Support Agent","description":""}`, adminCookie)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	templateID := decodeTemplateResponse(t, createRecorder.Body.Bytes()).Template.ID
	if recorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/soul", `{"content":"Original soul."}`, adminCookie); recorder.Code != http.StatusOK {
		t.Fatalf("put soul status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder := doJSON(t, router, http.MethodPut, "/api/admin/templates/"+templateID+"/user", `{"content":"Original user."}`, adminCookie); recorder.Code != http.StatusOK {
		t.Fatalf("put user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	return templateID
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
			soul_md_path TEXT NOT NULL,
			user_md_path TEXT NOT NULL,
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
