export const EXTENSION_SCENES = [
  {
    id: 'github',
    query: 'My github PRs',
    command: 'Show My Pull Requests',
    subtitle: 'List open pull requests across repos or current repo',
    title: 'My Pull Requests',
    searchStart: 448,
    typingStart: 454,
    moveStart: 478,
    click: 488,
    listStart: 494,
    end: 560,
  },
  {
    id: 'calendar',
    query: 'My Calendar Events',
    command: 'Upcoming Events',
    subtitle: 'Your calendar for the next 24 hours',
    title: 'Upcoming Events',
    searchStart: 560,
    typingStart: 566,
    moveStart: 593,
    click: 603,
    listStart: 609,
    end: 686,
  },
] as const;

export function existingExtensionAt(frame: number) {
  return EXTENSION_SCENES.find(
    (scene) => frame >= scene.searchStart && frame < scene.end,
  );
}
