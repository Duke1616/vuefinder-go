import type { Body, LocalUppyFile, Meta, RemoteUppyFile, UppyFile } from './UppyFile.js';
/**
 * Takes a file object and turns it into fileID, by converting file.name to lowercase,
 * removing extra characters and adding type, size and lastModified
 */
export default function generateFileID<M extends Meta, B extends Body>(file: Partial<Pick<UppyFile<M, B>, 'id' | 'type' | 'name'>> & Pick<UppyFile<M, B>, 'data'> & {
    meta?: {
        relativePath?: unknown;
    };
}, instanceId: string): string;
export type SafeFileIdBasis<M extends Meta, B extends Body> = Partial<Pick<UppyFile<M, B>, 'id' | 'type'>> & (Pick<RemoteUppyFile<M, B>, 'isRemote' | 'remote' | 'data'> | Pick<LocalUppyFile<M, B>, 'isRemote' | 'data'>) & {
    meta?: {
        relativePath?: unknown;
    } | undefined;
};
export declare function getSafeFileId<M extends Meta, B extends Body>(file: SafeFileIdBasis<M, B>, instanceId: string): string;
//# sourceMappingURL=generateFileID.d.ts.map