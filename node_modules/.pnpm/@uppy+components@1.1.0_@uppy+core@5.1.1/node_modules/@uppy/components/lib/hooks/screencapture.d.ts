import type { Uppy } from '@uppy/core';
import type { ScreenCaptureState } from '@uppy/screen-capture';
type ButtonProps = {
    type: 'button';
    onClick: () => void;
    disabled: boolean;
};
export type ScreenCaptureSnapshot = {
    state: ScreenCaptureState;
    stop: () => void;
    start: () => void;
    getVideoProps: () => {
        playsInline: boolean;
        autoPlay?: boolean;
        muted: boolean;
        controls?: boolean | undefined;
        src?: string;
    };
    getScreenshotButtonProps: () => ButtonProps;
    getRecordButtonProps: () => ButtonProps;
    getStopRecordingButtonProps: () => ButtonProps;
    getSubmitButtonProps: () => ButtonProps;
    getDiscardButtonProps: () => ButtonProps;
};
export type ScreenCaptureStore = {
    subscribe: (listener: () => void) => () => void;
    getSnapshot: () => ScreenCaptureSnapshot;
};
export declare function createScreenCaptureController(uppy: Uppy, onSubmit?: () => void): ScreenCaptureStore;
export {};
//# sourceMappingURL=screencapture.d.ts.map