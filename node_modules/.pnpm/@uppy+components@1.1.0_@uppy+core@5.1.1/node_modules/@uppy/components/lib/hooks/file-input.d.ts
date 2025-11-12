import type { NonNullableUppyContext } from '../types.js';
export type FileInputProps = {
    multiple?: boolean;
    accept?: string;
};
export type FileInputFunctions<EventType> = {
    getInputProps: () => {
        id: string;
        type: 'file';
        multiple: boolean;
        accept?: string;
        onChange: (event: EventType) => void;
    };
    getButtonProps: () => {
        type: 'button';
        onClick: () => void;
    };
};
export declare function createFileInput<EventType extends Event>(ctx: NonNullableUppyContext, props?: FileInputProps): FileInputFunctions<EventType>;
//# sourceMappingURL=file-input.d.ts.map