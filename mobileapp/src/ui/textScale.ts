// How far a piece of fixed chrome follows the reader's text size.
//
// Content (the terminal, the conversation, the answers) scales all the way. A label that
// lives in a fixed box — a segmented switch, a control pill, a count bubble — follows the
// text size up to the largest standard size (×1.35, "XXXL") and stops there. At the
// accessibility sizes such labels broke over two lines, ran off the right edge, or were cut
// off inside their box (simulator, 2026-10-05).
export const CHROME_MAX_SCALE = 1.35;
