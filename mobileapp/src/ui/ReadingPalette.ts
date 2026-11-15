import {createContext, useContext} from 'react';
import {paletteFor, type Palette} from './theme';
import type {MdColors} from './MarkdownView';
import {darkCodeBackground} from './codeReading';

// One resolved App Appearance palette, shared by Chat and its nested readers.
// Explicit ChatView palettes also support isolated/demo readers outside AppProvider.
export const ReadingPaletteContext = createContext<Palette>(paletteFor('dark'));
export const useReadingPalette = () => useContext(ReadingPaletteContext);

export function readingColors(pal: Palette) {
  const dark = darkCodeBackground(pal.bg);
  const link = dark ? '#27C7E6' : '#006F82';
  const markdown: MdColors = {text: pal.fg, dim: pal.fg2, code: pal.fg,
    codeBg: pal.raised, border: pal.divider, link};
  return {dark, link, markdown, userMarkdown: {...markdown, text: dark ? '#E6F7FB' : '#153944'},
    userBg: dark ? '#0E2B33' : '#DDF0F3'};
}
