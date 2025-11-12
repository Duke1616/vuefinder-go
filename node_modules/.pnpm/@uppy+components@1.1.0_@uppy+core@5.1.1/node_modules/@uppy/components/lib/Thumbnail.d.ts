import type { Body, Meta, UppyFile } from '@uppy/core';
import type { UppyContext } from './types.js';
export type ThumbnailProps = {
    file: UppyFile<Meta, Body>;
    width?: string;
    height?: string;
    images?: boolean;
    ctx?: UppyContext;
};
export default function Thumbnail(props: ThumbnailProps): import("preact").JSX.Element;
//# sourceMappingURL=Thumbnail.d.ts.map