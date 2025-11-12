import type Uppy from '@uppy/core';
import type { UploadStatus } from './types.js';
export declare function createUppyEventAdapter({ uppy, onStatusChange, onProgressChange, }: {
    uppy: Uppy;
    onStatusChange: (status: UploadStatus) => void;
    onProgressChange: (progress: number) => void;
}): {
    cleanup: () => void;
};
//# sourceMappingURL=uppyEventAdapter.d.ts.map