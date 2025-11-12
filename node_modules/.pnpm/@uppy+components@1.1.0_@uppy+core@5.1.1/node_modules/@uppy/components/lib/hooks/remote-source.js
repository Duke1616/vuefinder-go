import { dequal } from 'dequal/lite';
import { Subscribers } from './utils.js';
export function createRemoteSourceController(uppy, sourceId) {
    const plugin = uppy.getPlugin(sourceId);
    if (!plugin) {
        throw new Error(`(${sourceId}) is not installed. Install the plugin or the preset @uppy/remote-sources and add it to the Uppy instance`);
    }
    const subscribers = new Subscribers();
    const view = plugin.view;
    let didFirstRender = false;
    const onStateUpdate = (prev, next, patch) => {
        if (patch?.plugins?.[sourceId]) {
            subscribers.emit();
        }
    };
    // Keep a cached snapshot so that the reference stays stable when nothing
    // has changed, as expected by `useSyncExternalStore` from React
    let cachedSnapshot = {
        state: {
            ...plugin.getPluginState(),
            // By default the partialTree is an array of all folders you have opened at some point,
            // not the contents of the current folder. We filter it here to make it more intuitive to work with.
            partialTree: view.getDisplayedPartialTree(),
            selectedAmount: view.getSelectedAmount(),
            error: view.validateAggregateRestrictions(plugin.getPluginState().partialTree),
            breadcrumbs: view.getBreadcrumbs(),
        },
        login: view?.handleAuth,
        logout: view?.logout,
        open: view?.openFolder,
        checkbox: view?.toggleCheckbox,
        done: view?.donePicking,
        cancel: view?.cancelSelection,
    };
    const getSnapshot = () => {
        const nextSnapshot = {
            ...cachedSnapshot,
            state: {
                ...plugin.getPluginState(),
                partialTree: view.getDisplayedPartialTree(),
                selectedAmount: view.getSelectedAmount(),
                error: view.validateAggregateRestrictions(plugin.getPluginState().partialTree),
                breadcrumbs: view.getBreadcrumbs(),
            },
        };
        if (!dequal(cachedSnapshot.state, nextSnapshot.state)) {
            cachedSnapshot = nextSnapshot;
        }
        return cachedSnapshot;
    };
    const mount = () => {
        uppy.on('state-update', onStateUpdate);
        if (!didFirstRender) {
            view.openFolder?.(plugin.rootFolderId);
            view.provider.fetchPreAuthToken?.();
            didFirstRender = true;
        }
    };
    const unmount = () => {
        didFirstRender = false;
        uppy.off('state-update', onStateUpdate);
    };
    return {
        mount,
        unmount,
        subscribe: subscribers.add,
        getSnapshot,
    };
}
