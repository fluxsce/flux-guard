const tipClass = 'fg-chart-tooltip'

let depth = 0

function wheelPixels(event: WheelEvent, el: HTMLElement) {
  if (event.deltaMode === 1) return event.deltaY * 16
  if (event.deltaMode === 2) return event.deltaY * el.clientHeight
  return event.deltaY
}

/** 浮层挂在 body 上，滚轮默认会带动背后的监控页。这里把滚动留在浮层里。 */
function onWheel(event: WheelEvent) {
  const raw = event.target
  if (!(raw instanceof Element)) return
  const tip = raw.closest('.' + tipClass)
  if (!(tip instanceof HTMLElement)) return
  const max = tip.scrollHeight - tip.clientHeight
  if (max <= 1 || event.deltaY === 0) return
  const next = Math.min(max, Math.max(0, tip.scrollTop + wheelPixels(event, tip)))
  tip.scrollTop = next
  event.preventDefault()
  event.stopPropagation()
}

/**
 * 图表悬浮层可滚时，滚轮只滚动浮层，不带动背后页面。
 * 同一页多张图共用一次监听，全部卸载后再摘掉。
 * @returns 当前图表卸载时调用
 */
export function retainChartTooltipScroll(): () => void {
  if (typeof document === 'undefined') return () => {}
  if (depth === 0) {
    document.addEventListener('wheel', onWheel, { capture: true, passive: false })
  }
  depth += 1
  let released = false
  return () => {
    if (released) return
    released = true
    depth -= 1
    if (depth === 0) document.removeEventListener('wheel', onWheel, { capture: true })
  }
}
