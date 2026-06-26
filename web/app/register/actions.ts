import {
  registerUser,
  sendRegistrationEmailCode,
  type ApiClient,
} from "@/lib/api";

export async function sendRegisterEmailCode(apiClient: ApiClient, email: string) {
  return sendRegistrationEmailCode(apiClient, { email });
}

export async function registerWithPassword(
  apiClient: ApiClient,
  email: string,
  password: string,
  emailCode: string,
) {
  return registerUser(apiClient, { email, password, emailCode });
}
