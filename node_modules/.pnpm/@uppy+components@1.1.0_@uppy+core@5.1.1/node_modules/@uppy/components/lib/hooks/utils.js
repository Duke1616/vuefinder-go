export class Subscribers {
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
