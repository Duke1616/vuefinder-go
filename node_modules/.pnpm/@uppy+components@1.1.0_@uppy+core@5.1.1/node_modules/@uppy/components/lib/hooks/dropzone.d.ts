import type { NonNullableUppyContext } from '../types.js';
export type DropzoneOptions = {
    noClick?: boolean;
    onDragOver?: (event: Event) => void;
    onDragEnter?: (event: Event) => void;
    onDragLeave?: (event: Event) => void;
    onDrop?: (files: File[]) => void;
    onFileInputChange?: (files: File[]) => void;
};
export type DropzoneReturn<DragEventType, ChangeEventType> = {
    getRootProps: () => {
        onDragEnter: (event: DragEventType) => void;
        onDragOver: (event: DragEventType) => void;
        onDragLeave: (event: DragEventType) => void;
        onDrop: (event: DragEventType) => void;
        onClick: () => void;
    };
    getInputProps: () => {
        id: string;
        type: 'file';
        multiple: boolean;
        accept?: string;
        onChange: (event: ChangeEventType) => void;
    };
};
export declare function createDropzone<DragEventType extends DragEvent, ChangeEventType extends Event>(ctx: NonNullableUppyContext, options?: DropzoneOptions): DropzoneReturn<DragEventType, ChangeEventType>;
//# sourceMappingURL=dropzone.d.ts.map