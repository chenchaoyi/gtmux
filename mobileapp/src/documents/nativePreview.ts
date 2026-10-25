import {NativeModules} from 'react-native';
import {DocumentError} from './references';

// The transport/model are platform neutral. iOS supplies UIKit/WebKit; Android
// can implement this same module without changing document permissions or JSX.
export interface DocumentRequest {
  resolveURL: string;
  headers: Record<string, string>;
  reference: string;
  session_id: string;
  home_id: string;
  name: string;
  lang: string;
}
export interface DocumentPreviewAdapter {
  open(request: DocumentRequest): Promise<void>;
}

export const nativeDocumentPreview: DocumentPreviewAdapter = {
  async open(request) {
    const module = NativeModules.GtmuxDocuments;
    if (typeof module?.open !== 'function') throw new DocumentError('unsupported');
    await module.open(request);
  },
};
