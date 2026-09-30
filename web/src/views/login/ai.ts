import { getApiMessage, isApiSuccess } from '@/api/request'
import { useSessionStore } from '@/stores/session'
import { defineTools } from '@/ai/catalog'
import { FgAiError } from '@/ai/error'
import { argString } from '@/ai/validate'

defineTools([
  {
    name: 'login.signIn',
    moduleId: 'login',
    title: '登录',
    description: '使用用户 ID 和口令登录控制台。',
    risk: 'write',
    audience: 'anonymous',
    parameters: [
      { name: 'userId', type: 'string', description: '用户 ID', required: true },
      { name: 'password', type: 'string', description: '登录口令', required: true, secret: true },
    ],
    execute: async (args) => {
      const session = useSessionStore()
      const userId = argString(args, 'userId')
      const result = await session.signIn(userId, argString(args, 'password'))
      if (!isApiSuccess(result)) throw new FgAiError(getApiMessage(result, '登录失败'))
      return { userId, userName: session.user?.userName || '' }
    },
  },
])
