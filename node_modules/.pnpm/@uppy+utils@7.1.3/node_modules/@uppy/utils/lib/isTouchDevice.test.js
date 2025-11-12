import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import isTouchDevice from './isTouchDevice.js';
describe('isTouchDevice', () => {
    const RealTouchStart = globalThis.window.ontouchstart;
    const RealMaxTouchPoints = globalThis.navigator.maxTouchPoints;
    beforeEach(() => {
        // Set ontouchstart property to simulate touch device
        ;
        globalThis.window.ontouchstart = null;
        // Set maxTouchPoints to simulate touch device
        Object.defineProperty(globalThis.navigator, 'maxTouchPoints', {
            value: 1,
            configurable: true,
        });
    });
    afterEach(() => {
        // Restore original values
        if (RealMaxTouchPoints !== undefined) {
            Object.defineProperty(globalThis.navigator, 'maxTouchPoints', {
                value: RealMaxTouchPoints,
                configurable: true,
            });
        }
        else {
            delete globalThis.navigator.maxTouchPoints;
        }
        if (RealTouchStart !== undefined) {
            ;
            globalThis.window.ontouchstart = RealTouchStart;
        }
        else {
            delete globalThis.window.ontouchstart;
        }
    });
    it("should return true if it's a touch device", () => {
        expect(isTouchDevice()).toEqual(true);
        // Remove touch properties to simulate non-touch device
        delete globalThis.window.ontouchstart;
        delete globalThis.navigator.maxTouchPoints;
        expect(isTouchDevice()).toEqual(false);
    });
});
