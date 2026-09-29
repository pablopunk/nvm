export type NevermindDeviceSignInStatus =
  | { state: 'starting' }
  | {
      state: 'pending';
      verificationUrl: string;
      code: string;
      expiresAt: string;
      browserOpenFailed: boolean;
    }
  | { state: 'approved'; email: string }
  | { state: 'expired' }
  | { state: 'cancelled' }
  | { state: 'failed'; message: string };
