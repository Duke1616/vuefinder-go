import { jsx as _jsx, jsxs as _jsxs } from "preact/jsx-runtime";
import { clsx } from 'clsx';
import { Fragment } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import prettyBytes from 'pretty-bytes';
import { Thumbnail } from './index.js';
export default function FilesList(props) {
    const [files, setFiles] = useState(() => []);
    const { ctx, editFile, imageThumbnail } = props;
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
    return (_jsx("ul", { "data-uppy-element": "files-list", className: "uppy-reset uppy:my-4", children: files?.map((file) => (_jsx("li", { children: _jsxs(Fragment, { children: [_jsxs("div", { className: "uppy:flex uppy:items-center uppy:gap-2", children: [_jsx("div", { className: "uppy:w-[32px] uppy:h-[32px]", children: _jsx(Thumbnail, { width: "32px", height: "32px", file: file, images: imageThumbnail }) }), _jsx("p", { className: "uppy:truncate", children: file.name }), _jsx("p", { className: "uppy:text-gray-500 uppy:tabular-nums uppy:min-w-18 uppy:text-right uppy:ml-auto", children: prettyBytes(file.size || 0) }), _jsxs(Fragment, { children: [editFile && (_jsx("button", { type: "button", className: "uppy:flex uppy:rounded uppy:text-blue-500 uppy:hover:text-blue-700 uppy:bg-transparent uppy:transition-colors", onClick: () => {
                                            editFile(file);
                                        }, children: "edit" })), _jsx("button", { type: "button", className: "uppy:flex uppy:rounded uppy:text-blue-500 uppy:hover:text-blue-700 uppy:bg-transparent uppy:transition-colors", onClick: () => {
                                            ctx.uppy?.removeFile(file.id);
                                        }, children: "remove" })] })] }), _jsx("progress", { max: "100", className: clsx('uppy:w-full uppy:h-[2px] uppy:appearance-none uppy:bg-gray-100 uppy:rounded-full uppy:overflow-hidden uppy:[&::-webkit-progress-bar]:bg-gray-100 uppy:block uppy:my-2', {
                            'uppy:[&::-webkit-progress-value]:bg-green-500 uppy:[&::-moz-progress-bar]:bg-green-500': file.progress?.uploadComplete,
                            'uppy:[&::-webkit-progress-value]:bg-red-500 uppy:[&::-moz-progress-bar]:bg-red-500': file.error,
                            'uppy:[&::-webkit-progress-value]:bg-blue-500 uppy:[&::-moz-progress-bar]:bg-blue-500': !file.progress?.uploadComplete && !file.error,
                        }), value: file.progress?.percentage || 0 })] }) }, file.id))) }));
}
