import { jsx as _jsx, jsxs as _jsxs } from "preact/jsx-runtime";
import { clsx } from 'clsx';
export default function UploadButton(props) {
    const { ctx } = props;
    return (_jsxs("div", { className: "uppy-reset uppy:space-y-2", children: [_jsxs("button", { type: "button", "data-uppy-element": "upload-button", "data-state": ctx.status, onClick: () => {
                    if (ctx.status === 'ready') {
                        ctx.uppy?.upload();
                    }
                }, className: clsx('uppy:relative uppy:w-full uppy:p-2 uppy:rounded-lg', 'uppy:text-white uppy:font-medium', 'uppy:transition-all uppy:overflow-hidden', 'uppy:bg-blue-500 uppy:hover:bg-blue-600', {
                    'uppy:bg-red-500 uppy:hover:bg-red-600': ctx.status === 'error',
                    'uppy:bg-green-500 uppy:hover:bg-green-600': ctx.status === 'complete',
                }, 'uppy:disabled:hover:bg-blue-500 uppy:disabled:cursor-not-allowed'), disabled: ctx.status === 'init' ||
                    ctx.status === 'uploading' ||
                    ctx.status === 'paused', children: [_jsx("div", { className: clsx('uppy:absolute uppy:inset-0 uppy:origin-left uppy:transition-all', {
                            'uppy:bg-red-700': ctx.status === 'error',
                            'uppy:bg-green-700': ctx.status === 'complete',
                            'uppy:bg-blue-700': ctx.status !== 'error' && ctx.status !== 'complete',
                        }), style: {
                            transform: `scaleX(${ctx.progress / 100})`,
                        } }), _jsx("span", { className: "uppy:relative uppy:z-10", children: ctx.status === 'uploading' || ctx.status === 'paused'
                            ? `Uploaded ${Math.round(ctx.progress)}%`
                            : ctx.status === 'error'
                                ? 'Retry'
                                : ctx.status === 'complete'
                                    ? 'Complete'
                                    : 'Upload' })] }), ctx.status === 'uploading' || ctx.status === 'paused' ? (_jsxs("div", { className: "uppy:flex uppy:gap-2", children: [ctx.uppy?.getState().capabilities.resumableUploads ? (_jsx("button", { type: "button", "data-uppy-element": "pause-button", "data-state": ctx.status, onClick: () => {
                            if (ctx.status === 'paused') {
                                ctx.uppy?.resumeAll();
                            }
                            else {
                                ctx.uppy?.pauseAll();
                            }
                        }, className: clsx('uppy:w-full uppy:p-2 uppy:rounded-lg uppy:text-amber-500 uppy:bg-gray-50 uppy:hover:bg-amber-50 uppy:font-medium uppy:transition-all', {
                            'uppy:text-green-500 uppy:hover:bg-green-50': ctx.status === 'paused',
                        }), children: ctx.status === 'paused' ? 'Resume' : 'Pause' })) : null, _jsx("button", { type: "button", "data-uppy-element": "cancel-button", className: "uppy:w-full uppy:p-2 uppy:rounded-lg uppy:text-red-500 uppy:bg-gray-50 uppy:hover:bg-red-50 uppy:font-medium uppy:transition-all", "data-state": ctx.status, onClick: () => ctx.uppy?.cancelAll(), children: "Cancel" })] })) : null] }));
}
