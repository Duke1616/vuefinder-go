// Use a more generic constraint that works with both DOM Events and React/Vue Events
export function createFileInput(ctx, props = {}) {
    const fileInputId = `uppy-file-input-${ctx.uppy.getID()}`;
    const handleClick = () => {
        const input = document.getElementById(fileInputId);
        input?.click();
    };
    const handleFileInputChange = (event) => {
        const input = event.target;
        const files = Array.from(input.files || []);
        if (!files.length)
            return;
        ctx.uppy.addFiles(files.map((file) => ({
            name: file.name,
            type: file.type,
            data: file,
        })));
        // Reset the input value so the same file can be selected again
        input.value = '';
    };
    return {
        getInputProps: () => {
            const { restrictions } = ctx.uppy.opts;
            const { allowedFileTypes, maxNumberOfFiles } = restrictions;
            let accept = props.accept;
            accept ??= allowedFileTypes?.join(', ');
            let multiple = props.multiple;
            multiple ??= maxNumberOfFiles !== 1;
            return {
                id: fileInputId,
                type: 'file',
                multiple,
                accept,
                onChange: handleFileInputChange,
            };
        },
        getButtonProps: () => ({
            type: 'button',
            onClick: handleClick,
        }),
    };
}
