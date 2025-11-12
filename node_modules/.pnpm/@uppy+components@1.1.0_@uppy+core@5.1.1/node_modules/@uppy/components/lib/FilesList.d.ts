import type { Body, Meta, UppyFile } from '@uppy/core';
import { type UppyContext } from './index.js';
export type FilesListProps = {
    editFile?: (file: UppyFile<Meta, Body>) => void;
    imageThumbnail?: boolean;
    ctx: UppyContext;
};
export default function FilesList(props: FilesListProps): import("preact").JSX.Element;
//# sourceMappingURL=FilesList.d.ts.map