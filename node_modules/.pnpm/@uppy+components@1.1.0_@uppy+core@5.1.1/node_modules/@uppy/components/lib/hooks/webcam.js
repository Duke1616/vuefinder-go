import { Subscribers } from './utils.js';
const videoId = 'uppy-webcam-video';
export function createWebcamController(uppy, onSubmit) {
    const plugin = uppy.getPlugin('Webcam');
    if (!plugin) {
        throw new Error('Webcam plugin is not installed. Install @uppy/webcam and add it to the Uppy instance with `uppy.use(Webcam)`.');
    }
    const subscribers = new Subscribers();
    const onStateUpdate = (prev, next, patch) => {
        const webcamPatch = patch?.plugins?.Webcam;
        if (webcamPatch) {
            subscribers.emit();
        }
    };
    const stop = () => {
        uppy.off('state-update', onStateUpdate);
        if (plugin.webcamActive || plugin.getPluginState().isRecording) {
            plugin.stop();
        }
    };
    const start = () => {
        uppy.on('state-update', onStateUpdate);
        plugin.start();
    };
    const getVideoProps = () => {
        const ref = document.getElementById(videoId);
        plugin.getVideoElement = () => ref;
        const { recordedVideo, capturedSnapshot } = plugin.getPluginState();
        const status = plugin.getStatus();
        if (status === 'captured' && recordedVideo) {
            if (ref) {
                ref.srcObject = null;
            }
            return {
                id: videoId,
                'data-uppy-mirrored': false,
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
        if (status === 'captured' && capturedSnapshot) {
            if (ref) {
                ref.srcObject = null;
            }
            return {
                id: videoId,
                'data-uppy-mirrored': false,
                playsInline: true,
                controls: false,
                muted: true,
                poster: capturedSnapshot,
                autoPlay: undefined,
            };
        }
        if (ref && plugin.stream && !(capturedSnapshot || recordedVideo)) {
            ref.srcObject = plugin.stream;
        }
        // Live preview
        return {
            id: videoId,
            'data-uppy-mirrored': true,
            playsInline: true,
            autoPlay: true,
            muted: true,
            controls: undefined,
        };
    };
    const getSnapshotButtonProps = () => ({
        type: 'button',
        onClick: async () => {
            await plugin.takeSnapshot();
        },
        disabled: plugin.getStatus() !== 'ready' || plugin.getPluginState().isRecording,
    });
    const getRecordButtonProps = () => ({
        type: 'button',
        onClick: () => {
            plugin.startRecording();
        },
        disabled: plugin.getStatus() !== 'ready' || plugin.getPluginState().isRecording,
    });
    const getStopRecordingButtonProps = () => ({
        type: 'button',
        onClick: () => {
            plugin.stopRecording();
        },
        disabled: plugin.getStatus() !== 'recording',
    });
    const getSubmitButtonProps = () => ({
        type: 'button',
        onClick: () => {
            plugin.submit();
            plugin.stop();
            onSubmit?.();
        },
        disabled: plugin.getStatus() !== 'captured',
    });
    const getDiscardButtonProps = () => ({
        type: 'button',
        onClick: () => {
            plugin.discardRecordedMedia();
        },
        disabled: plugin.getStatus() !== 'captured',
    });
    const getSnapshot = () => ({
        state: plugin.getPluginState(),
        stop,
        start,
        getVideoProps,
        getSnapshotButtonProps,
        getRecordButtonProps,
        getStopRecordingButtonProps,
        getSubmitButtonProps,
        getDiscardButtonProps,
    });
    // Keep a cached snapshot so that the reference stays stable when nothing
    // has changed, as expected by `useSyncExternalStore` from React
    let cachedSnapshot = getSnapshot();
    const getCachedSnapshot = () => {
        const nextSnapshot = getSnapshot();
        // If the reference hasn't changed we need to return the cached
        // snapshot to avoid unnecessary re-renders.
        if (nextSnapshot.state === cachedSnapshot.state)
            return cachedSnapshot;
        cachedSnapshot = nextSnapshot;
        return cachedSnapshot;
    };
    return { subscribe: subscribers.add, getSnapshot: getCachedSnapshot };
}
