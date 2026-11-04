import React from 'react';
import type { ExtensionDetailLink } from '../../resources/nevermind-extension-api';

function hostLabel(url: string) {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

/** Compact Open Graph style card; shows the host immediately and fills in as metadata arrives. */
export function LinkCard({ link }: { link: ExtensionDetailLink }) {
  const site = link.siteName || hostLabel(link.url);
  if (link.status === 'unavailable' && !link.title) return null;
  return (
    <div className="extensionLinkCard" data-status={link.status}>
      {link.image ? (
        <img className="extensionLinkCardImage" src={link.image} alt="" />
      ) : null}
      <div className="extensionLinkCardBody">
        <small>{site}</small>
        <strong>
          {link.title || (link.status === 'loading' ? '…' : link.url)}
        </strong>
        {link.description ? <p>{link.description}</p> : null}
      </div>
    </div>
  );
}
