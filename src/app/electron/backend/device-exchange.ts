export type DeviceExchangeResponse = {
  status: string;
  token?: string;
  user?: { id?: string; email: string; role: string };
};

export function approvedDeviceExchange(data: DeviceExchangeResponse) {
  if (!(data.status === 'ok' && data.token && data.user)) {
    return null;
  }
  return {
    token: data.token,
    ...(data.user.id ? { userId: data.user.id } : {}),
    email: data.user.email,
    role: data.user.role,
  };
}
