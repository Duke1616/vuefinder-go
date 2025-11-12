import type { Uppy } from '@uppy/core';
import type { WebcamState, WebcamStatus } from '@uppy/webcam';
export type { WebcamStatus };
type ButtonProps = {
    type: 'button';
    onClick: () => void;
    disabled: boolean;
};
export type WebcamSnapshot = {
    state: WebcamState;
    stop: () => void;
    start: () => void;
    getVideoProps: () => {
        playsInline: boolean;
        autoPlay?: boolean;
        muted: boolean;
        controls?: boolean | undefined;
        src?: string;
    };
    getSnapshotButtonProps: () => ButtonProps;
    getRecordButtonProps: () => ButtonProps;
    getStopRecordingButtonProps: () => ButtonProps;
    getSubmitButtonProps: () => ButtonProps;
    getDiscardButtonProps: () => ButtonProps;
};
export type WebcamStore = {
    subscribe: (listener: () => void) => () => void;
    getSnapshot: () => WebcamSnapshot;
};
export declare function createWebcamController(uppy: Uppy, onSubmit?: () => void): WebcamStore;
//# sourceMappingURL=webcam.d.ts.map