import type { Body, Meta, UppyFile } from '@uppy/core';
import type { UppyContext } from './types.js';
export type FilesGridProps = {
    editFile?: (file: UppyFile<Meta, Body>) => void;
    columns?: number;
    imageThumbnail?: boolean;
    ctx: UppyContext;
};
export default function FilesGrid(props: FilesGridProps): import("preact").JSX.Element;
//# sourceMappingURL=FilesGrid.d.ts.map