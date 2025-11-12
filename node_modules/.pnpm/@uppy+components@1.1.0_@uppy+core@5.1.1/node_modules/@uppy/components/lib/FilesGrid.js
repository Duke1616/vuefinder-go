import { jsx as _jsx, jsxs as _jsxs } from "preact/jsx-runtime";
import { clsx } from 'clsx';
import { Fragment } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import prettyBytes from 'pretty-bytes';
import { Thumbnail } from './index.js';
export default function FilesGrid(props) {
    const [files, setFiles] = useState(() => []);
    const { ctx, editFile, imageThumbnail = true } = props;
    function gridColsClass() {
        return ({
            1: 'uppy:grid-cols-1',
            2: 'uppy:grid-cols-2',
            3: 'uppy:grid-cols-3',
            4: 'uppy:grid-cols-4',
            5: 'uppy:grid-cols-5',
            6: 'uppy:grid-cols-6',
        }[props.columns || 2] || 'uppy:grid-cols-2');
    }
    useEffect(() => {
        const onStateUpdate = (prev, next, patch) => {
            if (patch?.files) {
                setFiles(Object.values(patch.files));
            }
        };
        ctx.uppy?.on('state-update', onStateUpdate);
        return () => {
            ctx.uppy?.off('state-update', onStateUpdate);
        };
    }, [ctx.uppy]);
    return (_jsx("div", { "data-uppy-element": "files-grid", className: clsx('uppy:reset uppy:my-4 uppy:grid uppy:gap-4', gridColsClass()), children: files?.map((file) => (_jsx("div", { className: "uppy:flex uppy:flex-col uppy:items-center uppy:gap-2", children: _jsxs(Fragment, { children: [_jsx(Thumbnail, { images: imageThumbnail, file: file }), _jsxs("div", { className: "uppy:w-full uppy-reset", children: [_jsx("p", { className: "uppy:font-medium uppy:truncate", title: file.name, children: file.name }), _jsxs("div", { className: "uppy:flex uppy:items-center uppy:gap-2", children: [_jsx("p", { className: " uppy:text-gray-500 uppy:tabular-nums ", children: prettyBytes(file.size || 0) }), editFile && (_jsx("button", { type: "button", className: "uppy:flex uppy:rounded uppy:text-blue-500 uppy:hover:text-blue-700 uppy:bg-transparent uppy:transition-colors", onClick: () => {
                                            editFile(file);
                                        }, children: "edit" })), _jsx("button", { type: "button", className: "uppy:flex uppy:rounded uppy:text-blue-500 uppy:hover:text-blue-700 uppy:bg-transparent uppy:transition-colors", onClick: () => {
                                            ctx.uppy?.removeFile(file.id);
                                        }, children: "remove" })] })] })] }) }, file.id))) }));
}
