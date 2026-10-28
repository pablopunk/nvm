import type { WindowLayoutId } from '../electron/os/window-layout-catalog';

interface WindowLayoutPresentation {
  title: string;
  icon: string;
  keywords: string[];
}

function windowLayoutPresentation(
  title: string,
  icon: string,
  keywords: string[],
): WindowLayoutPresentation {
  return { title, icon, keywords };
}

export const WINDOW_LAYOUT_PRESENTATION: Record<
  WindowLayoutId,
  WindowLayoutPresentation
> = {
  'left-half': windowLayoutPresentation('Left Half', 'panel-left', [
    'left',
    'half',
  ]),
  'right-half': windowLayoutPresentation('Right Half', 'panel-right', [
    'right',
    'half',
  ]),
  'top-half': windowLayoutPresentation('Top Half', 'panel-top', [
    'top',
    'half',
  ]),
  'bottom-half': windowLayoutPresentation('Bottom Half', 'panel-bottom', [
    'bottom',
    'half',
  ]),
  'center-half': windowLayoutPresentation('Center Half', 'square-dashed', [
    'center',
    'middle',
    'half',
  ]),
  maximize: windowLayoutPresentation('Maximize', 'maximize', [
    'max',
    'full',
    'fill',
  ]),
  center: windowLayoutPresentation('Center', 'scan', ['middle', 'centre']),
  'first-third': windowLayoutPresentation('First Third', 'columns-3', [
    'left',
    'third',
    'column',
  ]),
  'center-third': windowLayoutPresentation('Center Third', 'columns-3', [
    'middle',
    'third',
    'column',
  ]),
  'last-third': windowLayoutPresentation('Last Third', 'columns-3', [
    'right',
    'third',
    'column',
  ]),
  'top-third': windowLayoutPresentation('Top Third', 'rows-3', [
    'top',
    'third',
    'row',
  ]),
  'middle-third': windowLayoutPresentation('Middle Third', 'rows-3', [
    'middle',
    'third',
    'row',
  ]),
  'bottom-third': windowLayoutPresentation('Bottom Third', 'rows-3', [
    'bottom',
    'third',
    'row',
  ]),
  'first-two-thirds': windowLayoutPresentation(
    'First Two Thirds',
    'columns-2',
    ['left', 'two-thirds', 'column'],
  ),
  'center-two-thirds': windowLayoutPresentation(
    'Center Two Thirds',
    'columns-2',
    ['middle', 'two-thirds', 'column'],
  ),
  'last-two-thirds': windowLayoutPresentation('Last Two Thirds', 'columns-2', [
    'right',
    'two-thirds',
    'column',
  ]),
  'top-two-thirds': windowLayoutPresentation('Top Two Thirds', 'rows-2', [
    'top',
    'two-thirds',
    'row',
  ]),
  'middle-two-thirds': windowLayoutPresentation('Middle Two Thirds', 'rows-2', [
    'middle',
    'two-thirds',
    'row',
  ]),
  'bottom-two-thirds': windowLayoutPresentation('Bottom Two Thirds', 'rows-2', [
    'bottom',
    'two-thirds',
    'row',
  ]),
  'top-left-quarter': windowLayoutPresentation('Top Left Quarter', 'grid-2x2', [
    'top',
    'left',
    'quarter',
  ]),
  'top-right-quarter': windowLayoutPresentation(
    'Top Right Quarter',
    'grid-2x2',
    ['top', 'right', 'quarter'],
  ),
  'bottom-left-quarter': windowLayoutPresentation(
    'Bottom Left Quarter',
    'grid-2x2',
    ['bottom', 'left', 'quarter'],
  ),
  'bottom-right-quarter': windowLayoutPresentation(
    'Bottom Right Quarter',
    'grid-2x2',
    ['bottom', 'right', 'quarter'],
  ),
  'first-fourth': windowLayoutPresentation('First Fourth', 'columns-4', [
    'first',
    'fourth',
    'column',
  ]),
  'second-fourth': windowLayoutPresentation('Second Fourth', 'columns-4', [
    'second',
    'fourth',
    'column',
  ]),
  'third-fourth': windowLayoutPresentation('Third Fourth', 'columns-4', [
    'third',
    'fourth',
    'column',
  ]),
  'last-fourth': windowLayoutPresentation('Last Fourth', 'columns-4', [
    'last',
    'fourth',
    'column',
  ]),
  'top-left-sixth': windowLayoutPresentation('Top Left Sixth', 'grid-3x3', [
    'top',
    'left',
    'sixth',
  ]),
  'top-center-sixth': windowLayoutPresentation('Top Center Sixth', 'grid-3x3', [
    'top',
    'middle',
    'sixth',
  ]),
  'top-right-sixth': windowLayoutPresentation('Top Right Sixth', 'grid-3x3', [
    'top',
    'right',
    'sixth',
  ]),
  'bottom-left-sixth': windowLayoutPresentation(
    'Bottom Left Sixth',
    'grid-3x3',
    ['bottom', 'left', 'sixth'],
  ),
  'bottom-center-sixth': windowLayoutPresentation(
    'Bottom Center Sixth',
    'grid-3x3',
    ['bottom', 'middle', 'sixth'],
  ),
  'bottom-right-sixth': windowLayoutPresentation(
    'Bottom Right Sixth',
    'grid-3x3',
    ['bottom', 'right', 'sixth'],
  ),
  'maximize-width': windowLayoutPresentation(
    'Maximize Width',
    'move-horizontal',
    ['width', 'full', 'horizontal'],
  ),
  'maximize-height': windowLayoutPresentation(
    'Maximize Height',
    'move-vertical',
    ['height', 'full', 'vertical'],
  ),
  'almost-maximize': windowLayoutPresentation('Almost Maximize', 'expand', [
    'almost',
    'max',
    'margin',
  ]),
  'reasonable-size': windowLayoutPresentation('Reasonable Size', 'app-window', [
    'reasonable',
    'size',
    'comfortable',
  ]),
  'move-left': windowLayoutPresentation('Move Left', 'move-left', [
    'left',
    'edge',
    'move',
  ]),
  'move-right': windowLayoutPresentation('Move Right', 'move-right', [
    'right',
    'edge',
    'move',
  ]),
  'move-up': windowLayoutPresentation('Move Up', 'move-up', [
    'up',
    'top',
    'edge',
    'move',
  ]),
  'move-down': windowLayoutPresentation('Move Down', 'move-down', [
    'down',
    'bottom',
    'edge',
    'move',
  ]),
  'make-larger': windowLayoutPresentation('Make Larger', 'zoom-in', [
    'larger',
    'bigger',
    'grow',
    'expand',
  ]),
  'make-smaller': windowLayoutPresentation('Make Smaller', 'zoom-out', [
    'smaller',
    'shrink',
    'reduce',
  ]),
  'next-display': windowLayoutPresentation('Next Display', 'monitor', [
    'display',
    'monitor',
    'screen',
    'next',
  ]),
  'previous-display': windowLayoutPresentation('Previous Display', 'monitor', [
    'display',
    'monitor',
    'screen',
    'previous',
  ]),
  restore: windowLayoutPresentation('Restore', 'undo-2', [
    'restore',
    'original',
    'back',
    'undo',
  ]),
  'toggle-full-screen': windowLayoutPresentation(
    'Toggle Full Screen',
    'fullscreen',
    ['fullscreen', 'full', 'screen', 'toggle'],
  ),
};
