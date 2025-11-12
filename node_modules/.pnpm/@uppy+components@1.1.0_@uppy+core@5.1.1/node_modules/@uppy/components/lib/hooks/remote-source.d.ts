import type Uppy from '@uppy/core';
import type { PartialTreeFolder, UnknownProviderPluginState } from '@uppy/core';
import type { ProviderViews } from '@uppy/provider-views';
export type RemoteSourceKeys = 'Box' | 'Dropbox' | 'Facebook' | 'GoogleDrive' | 'Instagram' | 'OneDrive' | 'Unsplash' | 'Url' | 'Zoom';
export type RemoteSourceSnapshot = {
    state: UnknownProviderPluginState & {
        breadcrumbs: PartialTreeFolder[];
        selectedAmount: number;
        error: string | null;
    };
    login: ProviderViews<any, any>['handleAuth'];
    logout: ProviderViews<any, any>['logout'];
    open: ProviderViews<any, any>['openFolder'];
    checkbox: ProviderViews<any, any>['toggleCheckbox'];
    done: ProviderViews<any, any>['donePicking'];
    cancel: ProviderViews<any, any>['cancelSelection'];
};
export type RemoteSourceStore = {
    subscribe: (listener: () => void) => () => void;
    getSnapshot: () => RemoteSourceSnapshot;
    mount: () => void;
    unmount: () => void;
};
export declare function createRemoteSourceController(uppy: Uppy, sourceId: RemoteSourceKeys): RemoteSourceStore;
//# sourceMappingURL=remote-source.d.ts.map