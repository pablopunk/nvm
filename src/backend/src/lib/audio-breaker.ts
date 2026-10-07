export async function handleTrippedAudioRoute(input: {
  prepared: boolean;
  releasePreparedReservation: () => Promise<void>;
  failDedup: () => Promise<void>;
}): Promise<void> {
  try {
    if (input.prepared) await input.releasePreparedReservation();
  } finally {
    await input.failDedup();
  }
}
