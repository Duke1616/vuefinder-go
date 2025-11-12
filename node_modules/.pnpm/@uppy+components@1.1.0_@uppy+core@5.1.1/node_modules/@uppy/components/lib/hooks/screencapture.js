const videoId = 'uppy-screencapture-video';
class Subscribers {
    subscribers = new Set();
    add = (listener) => {
        this.subscribers.add(listener);
        return () => this.subscribers.delete(listener);
    };
    emit = () => {
        for (const listener of this.subscribers) {
            listener();
        }
    };
    clear = () => {
        this.subscribers.clear();
    };
}
export function createScreenCaptureController(uppy, onSubmit) {
    const plugin = uppy.getPlugin('ScreenCapture');
    if (!plugin) {
        throw new Error('ScreenCapture plugin is not installed. Install @uppy/screen-capture and add it to the Uppy instance with `uppy.use(ScreenCapture)`.');
    }
    const subscribers = new Subscribers();
    const onStateUpdate = (prev, next, patch) => {
        const screenCapturePatch = patch?.plugins?.ScreenCapture;
        if (screenCapturePatch) {
            subscribers.emit();
        }
    };
    const stop = () => {
        uppy.off('state-update', onStateUpdate);
        plugin.stop();
    };
    const start = () => {
        uppy.on('state-update', onStateUpdate);
        plugin.start();
    };
    const getVideoProps = () => {
        const ref = document.getElementById(videoId);
        const { status, recordedVideo, capturedScreenshotUrl } = plugin.getPluginState();
        if (status === 'captured' && recordedVideo) {
            if (ref) {
                ref.srcObject = null;
            }
            return {
                id: videoId,
                playsInline: true,
                controls: true,
                muted: false,
                src: recordedVideo,
                autoPlay: undefined,
            };
        }
        /*
         * If the status is 'captured', we use the capturedScreenshotUrl as the poster
         * for the video element. This allows us to display the screenshot as a static image
         * disabling controls and autoplay helps disguise the video element
         * as a result we're using the same video element for both the screenshot and the recorded video.
         */
        if (status === 'captured' && capturedScreenshotUrl) {
            if (ref) {
                ref.srcObject = null;
            }
            return {
                id: videoId,
                playsInline: true,
                controls: false,
                muted: false,
                poster: capturedScreenshotUrl,
                autoPlay: undefined,
            };
        }
        if (ref &&
            plugin.videoStream &&
            !(capturedScreenshotUrl || recordedVideo)) {
            ref.srcObject = plugin.videoStream;
        }
        return {
            id: videoId,
            playsInline: true,
            autoPlay: true,
            muted: true,
            controls: undefined,
        };
    };
    const getScreenshotButtonProps = () => {
        const { status } = plugin.getPluginState();
        return {
            type: 'button',
            onClick: async () => {
                await plugin.captureScreenshot();
            },
            disabled: status !== 'ready',
        };
    };
    const getRecordButtonProps = () => {
        const { status } = plugin.getPluginState();
        return {
            type: 'button',
            onClick: () => {
                plugin.startRecording();
            },
            disabled: status !== 'ready',
        };
    };
    const getStopRecordingButtonProps = () => {
        const { status } = plugin.getPluginState();
        return {
            type: 'button',
            onClick: () => {
                plugin.stopRecording();
            },
            disabled: status !== 'recording',
        };
    };
    const getSubmitButtonProps = () => {
        const { status } = plugin.getPluginState();
        return {
            type: 'button',
            onClick: () => {
                plugin.submit();
                plugin.stop();
                onSubmit?.();
            },
            disabled: !(status === 'captured'),
        };
    };
    const getDiscardButtonProps = () => {
        const { status } = plugin.getPluginState();
        return {
            type: 'button',
            onClick: () => {
                plugin.discardRecordedMedia();
            },
            disabled: !(status === 'captured'),
        };
    };
    const getSnapshot = () => ({
        state: plugin.getPluginState(),
        stop,
        start,
        getVideoProps,
        getScreenshotButtonProps,
        getRecordButtonProps,
        getStopRecordingButtonProps,
        getSubmitButtonProps,
        getDiscardButtonProps,
    });
    let cachedSnapshot = getSnapshot();
    const getCachedSnapshot = () => {
        const nextSnapshot = getSnapshot();
        if (nextSnapshot.state === cachedSnapshot.state)
            return cachedSnapshot;
        cachedSnapshot = nextSnapshot;
        return cachedSnapshot;
    };
    return { subscribe: subscribers.add, getSnapshot: getCachedSnapshot };
}
