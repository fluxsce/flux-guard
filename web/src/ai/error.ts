/**
 * 助手调用被拒绝或参数不合法时抛出。业务失败同样用这个错误把接口文案交回去。
 */
export class FgAiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'FgAiError'
  }
}
