export declare class Subscribers {
    private subscribers;
    add: (listener: () => void) => (() => void);
    emit: () => void;
    clear: () => void;
}
//# sourceMappingURL=utils.d.ts.map