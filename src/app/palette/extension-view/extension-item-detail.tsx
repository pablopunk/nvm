import type { ReactNode } from 'react';
import type { CommandItem } from '../model';
import { themedImageSource, useSystemTheme } from '../system-theme';
import type { ExtensionViewRendererProps } from './extension-view';
import { LinkCard } from './link-card';

function MetadataRows({
  items = [],
}: {
  items?: NonNullable<CommandItem['detail']>['metadata'];
}) {
  if (!items?.length) return null;
  return (
    <dl className="extensionMetadata">
      {items.map((item, index) => {
        if (item.type === 'separator')
          return <div key={index} className="metadataSeparator" />;
        if (item.type === 'tag')
          return (
            <div key={index} className="metadataTagRow">
              <dt>{item.label || 'Tag'}</dt>
              <dd>
                <span
                  className="accessory metadataTag"
                  data-tone={item.tone || 'default'}
                >
                  {item.value}
                </span>
              </dd>
            </div>
          );
        if (item.type === 'link')
          return (
            <div key={index}>
              <dt>{item.label}</dt>
              <dd>
                <a href={item.url}>{item.value}</a>
              </dd>
            </div>
          );
        return (
          <div key={index}>
            <dt>{item.label}</dt>
            <dd>{item.value}</dd>
          </div>
        );
      })}
    </dl>
  );
}

export function ExtensionItemDetail({
  item,
  renderMarkdown,
  renderActionPanel,
  actionPanelRows,
  mediaPreview = false,
}: {
  item?: CommandItem;
  renderMarkdown: (
    content: string,
    options?: { remoteImages?: 'links' },
  ) => ReactNode;
  renderActionPanel: (rows: unknown[], emptyMessage?: string) => ReactNode;
  actionPanelRows: ExtensionViewRendererProps['actionPanelRows'];
  mediaPreview?: boolean;
}) {
  const theme = useSystemTheme();
  const detail = item?.detail;
  if (!(item && detail)) return null;
  const actions = detail.actions?.length
    ? renderActionPanel(
        actionPanelRows(
          { sections: [{ actions: detail.actions }] },
          [],
          `detail-${item.id}`,
          false,
        ),
      )
    : null;
  const image = themedImageSource(detail.image || item.image, theme);
  const video = mediaPreview ? detail.video : undefined;
  const textOnlyPreview =
    mediaPreview && detail.text !== undefined && !image && !video;
  const linkCard = detail.link ? <LinkCard link={detail.link} /> : null;
  return (
    <aside
      className={`extensionDetailPane ${mediaPreview ? 'sidePreviewDetailPane' : ''} ${textOnlyPreview ? 'sidePreviewTextOnly' : ''} ${linkCard ? 'sidePreviewWithLink' : ''}`}
    >
      {textOnlyPreview ? null : (
        <div className="extensionDetailHeader">
          {image && !mediaPreview ? <img src={image} alt="" /> : null}
          <div>
            <strong>{detail.title || item.title}</strong>
            {detail.subtitle || item.subtitle ? (
              <small>{detail.subtitle || item.subtitle}</small>
            ) : null}
          </div>
        </div>
      )}
      {mediaPreview ? (
        <div className="extensionDetailPreview">
          {video ? (
            <video
              className="extensionDetailMedia"
              src={video}
              poster={image || undefined}
              controls={true}
              autoPlay={true}
              muted={true}
              loop={true}
              playsInline={true}
            />
          ) : image ? (
            <img className="extensionDetailMedia" src={image} alt="" />
          ) : detail.text === undefined ? (
            detail.markdown ? (
              <div className="previewText detailMarkdown">
                {renderMarkdown(detail.markdown)}
              </div>
            ) : null
          ) : (
            <pre className="previewText extensionDetailText">{detail.text}</pre>
          )}
          {linkCard}
        </div>
      ) : (
        <>
          {detail.markdown ? (
            <div className="previewText detailMarkdown">
              {renderMarkdown(detail.markdown)}
            </div>
          ) : null}
          {detail.text === undefined ? null : (
            <pre className="previewText extensionDetailText">{detail.text}</pre>
          )}
        </>
      )}
      <MetadataRows items={detail.metadata} />
      {actions}
    </aside>
  );
}
