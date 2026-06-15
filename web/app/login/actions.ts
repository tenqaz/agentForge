import type { ApiClient } from "@/lib/api";
import type { UserResponse } from "@/lib/api";

export async function signInWithPassword(
  apiClient: ApiClient,
  email: string,
  password: string,
) {
  return apiClient.post<UserResponse, { email: string; password: string }>(
    "/api/sessions",
    {
      email,
      password,
    },
  );
}
