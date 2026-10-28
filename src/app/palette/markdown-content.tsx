import React, { type ReactNode } from 'react';
import ReactMarkdown, {
  type Components,
  defaultUrlTransform,
} from 'react-markdown';
import remarkGfm from 'remark-gfm';

export type MarkdownRemoteImages = 'render' | 'links';

export type MarkdownOptions = { remoteImages?: 'links' };

export const AI_CHAT_MARKDOWN_OPTIONS: MarkdownOptions = {
  remoteImages: 'links',
};

export interface MarkdownContentProps {
  content: string;
  remoteImages?: MarkdownRemoteImages;
}

function MarkdownAnchor({
  children,
  href,
}: {
  children?: ReactNode;
  href?: string;
}) {
  return (
    <a href={href} target="_blank" rel="noreferrer">
      {children}
    </a>
  );
}

function MarkdownImageLink({ src, alt }: { src?: string; alt?: string }) {
  return <MarkdownAnchor href={src}>{alt || src}</MarkdownAnchor>;
}

function markdownImageComponents(
  remoteImages: MarkdownRemoteImages,
): Components {
  return remoteImages === 'links' ? { img: MarkdownImageLink } : {};
}

export function MarkdownContent({
  content,
  remoteImages = 'render',
}: MarkdownContentProps) {
  return (
    <div className="markdownContent">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        urlTransform={defaultUrlTransform}
        components={{
          h1: ({ children }) => (
            <h1 className="markdownHeading markdownHeading1">{children}</h1>
          ),
          h2: ({ children }) => (
            <h2 className="markdownHeading markdownHeading2">{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 className="markdownHeading markdownHeading3">{children}</h3>
          ),
          h4: ({ children }) => (
            <h4 className="markdownHeading markdownHeading4">{children}</h4>
          ),
          h5: ({ children }) => (
            <h5 className="markdownHeading markdownHeading5">{children}</h5>
          ),
          h6: ({ children }) => (
            <h6 className="markdownHeading markdownHeading6">{children}</h6>
          ),
          a: MarkdownAnchor,
          ...markdownImageComponents(remoteImages),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}
