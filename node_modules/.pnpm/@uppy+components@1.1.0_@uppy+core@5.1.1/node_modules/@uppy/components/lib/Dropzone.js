import { jsx as _jsx, jsxs as _jsxs } from "preact/jsx-runtime";
import { clsx } from 'clsx';
import { useMemo } from 'preact/hooks';
import { createDropzone } from './hooks/dropzone.js';
export default function Dropzone(props) {
    const { width, height, note, noClick, ctx } = props;
    if (!ctx.uppy) {
        throw new Error('Dropzone must be used within a UppyContextProvider');
    }
    const { getRootProps, getInputProps } = useMemo(() => createDropzone(ctx, { noClick }), [ctx, noClick]);
    return (_jsxs("div", { className: "uppy-reset", "data-uppy-element": "dropzone", role: "presentation", children: [_jsx("input", { ...getInputProps(), tabIndex: -1, name: "uppy-dropzone-file-input", className: "uppy:hidden" }), _jsxs("div", { ...getRootProps(), 
                // biome-ignore lint/a11y/noNoninteractiveTabindex: it is also a button. getRootProps returns keyboard event handlers
                tabIndex: 0, style: {
                    width: width || '100%',
                    height: height || '100%',
                }, className: clsx('uppy:border-2 uppy:border-dashed uppy:border-gray-300', 'uppy:rounded-lg uppy:p-6 uppy:bg-gray-50', 'uppy:transition-colors uppy:duration-200', {
                    'uppy:cursor-pointer uppy:hover:bg-blue-50': !noClick,
                }), children: [_jsx("div", { className: "uppy:flex uppy:flex-col uppy:items-center uppy:justify-center uppy:h-full uppy:space-y-3", children: _jsx("p", { className: "uppy:text-gray-600", children: "Drop files here or click to add them" }) }), note ? (_jsx("div", { className: "uppy:text-sm uppy:text-gray-500", children: note })) : null] })] }));
}
