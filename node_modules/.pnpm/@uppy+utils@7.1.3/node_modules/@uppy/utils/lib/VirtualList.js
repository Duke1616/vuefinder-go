import { jsx as _jsx } from "preact/jsx-runtime";
import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
const STYLE_INNER = {
    position: 'relative',
    // Disabled for our use case: the wrapper elements around FileList already deal with overflow,
    // and this additional property would hide things that we want to show.
    //
    // overflow: 'hidden',
    width: '100%',
    minHeight: '100%',
};
const STYLE_CONTENT = {
    position: 'absolute',
    top: 0,
    left: 0,
    // Because the `top` value gets set to some offset, this `height` being 100% would make the scrollbar
    // stretch far beyond the content. For our use case, the content div actually can get its height from
    // the elements inside it, so we don't need to specify a `height` property at all.
    //
    // height: '100%',
    width: '100%',
    overflow: 'visible',
};
export default function VirtualList({ data, rowHeight, renderRow, overscanCount = 10, padding = 4, ...props }) {
    const scrollerRef = useRef(null);
    const [offset, setOffset] = useState(0);
    const [height, setHeight] = useState(0);
    useEffect(() => {
        function resize() {
            if (scrollerRef.current != null &&
                height !== scrollerRef.current.offsetHeight) {
                setHeight(scrollerRef.current.offsetHeight);
            }
        }
        resize();
        window.addEventListener('resize', resize);
        return () => {
            window.removeEventListener('resize', resize);
        };
    }, [height]);
    const handleScroll = useCallback(() => {
        if (scrollerRef.current)
            setOffset(scrollerRef.current.scrollTop);
    }, []);
    // first visible row index
    let start = Math.floor(offset / rowHeight);
    // actual number of visible rows (without overscan)
    let visibleRowCount = Math.floor(height / rowHeight);
    // Overscan: render blocks of rows modulo an overscan row count
    // This dramatically reduces DOM writes during scrolling
    if (overscanCount) {
        start = Math.max(0, start - (start % overscanCount));
        visibleRowCount += overscanCount;
    }
    const end = start + visibleRowCount + padding;
    // data slice currently in viewport plus overscan items
    const selection = data.slice(start, end);
    const styleInner = { ...STYLE_INNER, height: data.length * rowHeight };
    const styleContent = { ...STYLE_CONTENT, top: start * rowHeight };
    // The `role="presentation"` attributes ensure that these wrapper elements are not treated as list
    // items by accessibility and outline tools.
    return (_jsx("div", { onScroll: handleScroll, ref: scrollerRef, ...props, children: _jsx("div", { role: "presentation", style: styleInner, children: _jsx("div", { role: "presentation", style: styleContent, children: selection.map(renderRow) }) }) }));
}
