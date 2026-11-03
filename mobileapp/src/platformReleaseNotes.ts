// Platform release archives share the same reader and acknowledgement model.
import {Platform} from 'react-native';
import {RELEASE_NOTES, ReleaseNote} from './releaseNotes';
import androidNotes from '../release-notes/android.json';

export function notesForPlatform(platform: string): ReleaseNote[] {
  return platform === 'android' ? androidNotes : RELEASE_NOTES;
}
export const MOBILE_RELEASE_NOTES = notesForPlatform(Platform.OS);
