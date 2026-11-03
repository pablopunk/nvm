import { clipboardItemSubtitle, clipboardItemTitle } from './clipboard-utils';
import { type LinkPreview, singleWebUrl } from './link-preview';

/** Side-preview detail for a history entry; web URLs also carry a link card. */
export function clipboardItemDetail(
  item: any,
  linkPreview: (url: string) => LinkPreview,
) {
  const base = {
    title: clipboardItemTitle(item),
    subtitle: clipboardItemSubtitle(item),
  };
  if (item.type === 'image')
    return { ...base, image: item.imageDataUrl || item.thumbnailUrl };
  if (item.type === 'video')
    return { ...base, image: item.thumbnailUrl, video: item.videoUrl };
  const url = singleWebUrl(item.text);
  return {
    ...base,
    text: item.text,
    ...(url ? { link: linkPreview(url) } : {}),
  };
}
