import {
  registerUser,
  sendRegistrationEmailCode,
  type ApiClient,
} from "@/lib/api";

export async function sendRegisterEmailCode(apiClient: ApiClient, email: string, turnstileToken: string) {
  return sendRegistrationEmailCode(apiClient, { email, turnstileToken });
}

export async function registerWithPassword(
  apiClient: ApiClient,
  email: string,
  password: string,
  emailCode: string,
  turnstileToken: string,
) {
  return registerUser(apiClient, { email, password, emailCode, turnstileToken });
}
