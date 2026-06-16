import { registerUser, type ApiClient } from "@/lib/api";

export async function registerWithPassword(
  apiClient: ApiClient,
  email: string,
  password: string,
) {
  return registerUser(apiClient, { email, password });
}
