import React, { type ReactNode } from 'react';
import type { CommandImage } from '../model';
import { useSystemTheme } from '../system-theme';
import { imageProps } from '../ui';

export interface PreviewViewProps {
  content?: ReactNode;
  image?: CommandImage;
  video?: string;
  poster?: string;
  actions?: ReactNode;
}

export function PreviewView({
  content,
  image,
  video,
  poster,
  actions,
}: PreviewViewProps) {
  const theme = useSystemTheme();
  const media = imageProps(image, theme);
  return (
    <div className="extensionView previewView">
      {video ? (
        <video
          className="previewMedia"
          src={video}
          poster={poster || media?.src}
          controls={true}
          autoPlay={true}
          muted={true}
          loop={true}
          playsInline={true}
        />
      ) : null}
      {!video && media?.src ? (
        <img
          className="previewMedia"
          src={media.src}
          alt={media.alt}
          data-fit={media.fit}
          data-shape={media.shape}
        />
      ) : null}
      <div className="previewText">{content}</div>
      {actions}
    </div>
  );
}
