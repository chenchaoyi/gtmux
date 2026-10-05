import type {ModalProps} from 'react-native';

// Every <Modal> declares the orientations the app runs in. React Native's Modal on iOS
// defaults to portrait only, so a sheet opened with the iPhone held sideways turned the
// whole screen upright until it closed (%6, 2026-10-06: 874×402 became 402×874 for New
// session and the snippets panel). Info.plist allows portrait and both landscapes on the
// iPhone and all four on the iPad; iOS intersects this list with those, so one list
// serves both.
export const MODAL_ORIENTATIONS: ModalProps['supportedOrientations'] = ['portrait', 'portrait-upside-down', 'landscape-left', 'landscape-right'];
