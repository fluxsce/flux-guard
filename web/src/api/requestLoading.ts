/**
 * 请求顶栏进度。并发请求共用一条：第一条才 start，全部结束才 finish。
 */
export interface RequestLoadingBar {
  start: () => void
  finish: () => void
}

export class RequestLoadingHelper {
  private count = 0
  private bar: RequestLoadingBar | null = null

  bind(bar: RequestLoadingBar): void {
    this.bar = bar
  }

  begin(): void {
    if (this.count === 0 && this.bar) {
      this.bar.start()
    }
    this.count++
  }

  end(showLoading?: boolean): void {
    if (!showLoading) {
      return
    }
    if (this.count <= 0) {
      return
    }
    this.count--
    if (this.count === 0 && this.bar) {
      this.bar.finish()
    }
  }
}

export const requestLoadingHelper = new RequestLoadingHelper()

export function initRequestTools(loadingBarInstance: RequestLoadingBar): void {
  requestLoadingHelper.bind(loadingBarInstance)
}
