<script setup lang="ts">
import { computed } from 'vue'
import {
  applyPlaceholders,
  channelLabels,
  contentLabels,
  imBrand,
  previewSkin,
  previewSrcdoc,
  sampleVars,
} from './previewRender'

const props = withDefaults(
  defineProps<{
    subject?: string
    body?: string
    contentType?: string
    channelKind?: string
    /** 为 true 时用样例数据替换 {{name}}，编辑态本地预览用。 */
    sample?: boolean
    compact?: boolean
  }>(),
  {
    subject: '',
    body: '',
    contentType: 'text',
    channelKind: 'webhook',
    sample: false,
    compact: false,
  },
)

const subjectText = computed(() => (props.sample ? applyPlaceholders(props.subject) : props.subject))
const skin = computed(() => previewSkin(props.channelKind))
const brand = computed(() => imBrand(props.channelKind))
const channelLabel = computed(() => channelLabels[props.channelKind] || props.channelKind || '渠道')
const contentLabel = computed(() => contentLabels[props.contentType] || props.contentType || '文本')
const frameDoc = computed(() =>
  previewSrcdoc(props.contentType, props.body, {
    sample: props.sample,
    channelKind: props.channelKind,
  }),
)
const nowLabel = computed(() => {
  const d = new Date()
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
})
</script>

<template>
  <div class="tp" :class="[`tp--${skin}`, compact ? 'tp--compact' : '']" :style="{ '--tp-accent': brand.accent }">
    <div class="tp__meta">
      <span class="tp__chip">{{ channelLabel }}</span>
      <span class="tp__chip tp__chip--muted">{{ contentLabel }}</span>
      <span v-if="sample" class="tp__chip tp__chip--muted">样例数据</span>
    </div>

    <!-- 邮件客户端 -->
    <div v-if="skin === 'email'" class="tp-shell tp-mail">
      <div class="tp-mail__chrome">
        <span class="tp-dot tp-dot--r" /><span class="tp-dot tp-dot--y" /><span class="tp-dot tp-dot--g" />
        <span class="tp-mail__title">收件箱 · 预警通知</span>
      </div>
      <div class="tp-mail__headers">
        <div class="tp-mail__row"><span>发件人</span><b>FluxGuard &lt;alert@fluxguard.local&gt;</b></div>
        <div class="tp-mail__row"><span>收件人</span><b>{{ sampleVars.contactNames }} &lt;oncall@example.com&gt;</b></div>
        <div class="tp-mail__row tp-mail__row--subject"><span>主题</span><b>{{ subjectText || '（无标题）' }}</b></div>
      </div>
      <iframe class="tp__frame tp__frame--mail" sandbox="" title="邮件正文预览" :srcdoc="frameDoc" />
    </div>

    <!-- 即时通讯卡片 -->
    <div v-else-if="skin === 'im'" class="tp-shell tp-im">
      <div class="tp-im__bar">
        <span class="tp-im__avatar" aria-hidden="true">{{ brand.name.slice(0, 1) }}</span>
        <div class="tp-im__who">
          <div class="tp-im__name">{{ brand.bot }}</div>
          <div class="tp-im__sub">{{ brand.name }} · {{ nowLabel }}</div>
        </div>
      </div>
      <div class="tp-im__card">
        <div v-if="subjectText" class="tp-im__subject">{{ subjectText }}</div>
        <iframe class="tp__frame tp__frame--im" sandbox="" title="消息正文预览" :srcdoc="frameDoc" />
      </div>
    </div>

    <!-- 短信 -->
    <div v-else-if="skin === 'sms'" class="tp-shell tp-sms">
      <div class="tp-sms__phone">
        <div class="tp-sms__notch" />
        <div class="tp-sms__head">短信 · {{ sampleVars.hostName }}</div>
        <div class="tp-sms__thread">
          <div class="tp-sms__bubble">
            <div v-if="subjectText" class="tp-sms__subject">{{ subjectText }}</div>
            <iframe class="tp__frame tp__frame--sms" sandbox="" title="短信正文预览" :srcdoc="frameDoc" />
          </div>
          <div class="tp-sms__time">{{ nowLabel }}</div>
        </div>
      </div>
    </div>

    <!-- Webhook -->
    <div v-else class="tp-shell tp-hook">
      <div class="tp-hook__chrome">
        <span class="tp-hook__method">POST</span>
        <code class="tp-hook__url">/hooks/alert</code>
        <span class="tp-hook__status">200</span>
      </div>
      <div v-if="subjectText" class="tp-hook__subject">{{ subjectText }}</div>
      <iframe class="tp__frame tp__frame--hook" sandbox="" title="Webhook 正文预览" :srcdoc="frameDoc" />
    </div>
  </div>
</template>

<style scoped>
.tp {
  --tp-accent: #64748b;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.tp__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.tp__chip {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--tp-accent);
  background: color-mix(in srgb, var(--tp-accent) 12%, transparent);
}
.tp__chip--muted {
  color: var(--rs-muted, #6b7280);
  background: var(--rs-surface-muted, #f3f4f6);
  font-weight: 500;
}
.tp-shell {
  border: 1px solid var(--rs-border, #e5e7eb);
  border-radius: 12px;
  overflow: hidden;
  background: var(--rs-surface, #fff);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}
.tp__frame {
  display: block;
  width: 100%;
  border: 0;
  background: transparent;
}
.tp__frame--mail { height: 220px; background: #fff; }
.tp__frame--im { height: 180px; }
.tp__frame--sms { height: 140px; }
.tp__frame--hook { height: 180px; background: #0f172a; }
.tp--compact .tp__frame--mail { height: 200px; }
.tp--compact .tp__frame--im { height: 168px; }
.tp--compact .tp__frame--sms { height: 128px; }
.tp--compact .tp__frame--hook { height: 160px; }

.tp-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
}
.tp-dot--r { background: #ff5f57; }
.tp-dot--y { background: #febc2e; }
.tp-dot--g { background: #28c840; }

.tp-mail__chrome {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: linear-gradient(180deg, #f8fafc, #f1f5f9);
  border-bottom: 1px solid var(--rs-border, #e5e7eb);
}
.tp-mail__title {
  margin-left: 8px;
  font-size: 12px;
  color: var(--rs-muted, #64748b);
}
.tp-mail__headers {
  display: grid;
  gap: 0;
  padding: 10px 14px;
  border-bottom: 1px solid var(--rs-border, #e5e7eb);
  background: #fff;
}
.tp-mail__row {
  display: grid;
  grid-template-columns: 52px 1fr;
  gap: 8px;
  align-items: baseline;
  padding: 3px 0;
  font-size: 12px;
}
.tp-mail__row > span {
  color: var(--rs-muted, #94a3b8);
}
.tp-mail__row > b {
  font-weight: 550;
  color: #0f172a;
  overflow-wrap: anywhere;
}
.tp-mail__row--subject > b {
  font-size: 13px;
  font-weight: 650;
}

.tp-im {
  background: #edf1f5;
  padding: 14px;
}
.tp-im__bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.tp-im__avatar {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  background: var(--tp-accent);
  box-shadow: 0 2px 6px color-mix(in srgb, var(--tp-accent) 35%, transparent);
}
.tp-im__name {
  font-size: 13px;
  font-weight: 650;
  color: #1f2329;
}
.tp-im__sub {
  font-size: 11px;
  color: #8a919f;
  margin-top: 1px;
}
.tp-im__card {
  background: #fff;
  border-radius: 10px;
  border: 1px solid rgba(15, 23, 42, 0.06);
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.06);
  overflow: hidden;
}
.tp-im__subject {
  padding: 12px 14px 0;
  font-size: 13px;
  font-weight: 650;
  color: #1f2329;
  overflow-wrap: anywhere;
}
.tp-im__card .tp__frame--im {
  padding: 8px 6px 10px;
}

.tp-sms {
  display: flex;
  justify-content: center;
  background: linear-gradient(180deg, #e8edf3, #dfe6ee);
  padding: 16px 12px 18px;
}
.tp-sms__phone {
  width: min(100%, 280px);
  border-radius: 28px;
  background: #111827;
  padding: 10px 10px 14px;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.22);
}
.tp-sms__notch {
  width: 72px;
  height: 6px;
  border-radius: 999px;
  background: #374151;
  margin: 4px auto 10px;
}
.tp-sms__head {
  text-align: center;
  color: #9ca3af;
  font-size: 11px;
  margin-bottom: 12px;
}
.tp-sms__thread {
  background: #f3f4f6;
  border-radius: 18px;
  padding: 14px 12px;
  min-height: 180px;
}
.tp-sms__bubble {
  max-width: 100%;
  background: #fff;
  border-radius: 14px 14px 14px 4px;
  padding: 10px 12px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
}
.tp-sms__subject {
  font-size: 12px;
  font-weight: 650;
  margin-bottom: 4px;
  color: #111827;
  overflow-wrap: anywhere;
}
.tp-sms__time {
  margin-top: 8px;
  text-align: center;
  font-size: 10px;
  color: #9ca3af;
}

.tp-hook {
  background: #0b1220;
  border-color: #1e293b;
}
.tp-hook__chrome {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid #1e293b;
  background: #111827;
}
.tp-hook__method {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #86efac;
  background: rgba(134, 239, 172, 0.12);
  border-radius: 4px;
  padding: 2px 6px;
}
.tp-hook__url {
  flex: 1;
  min-width: 0;
  font: 11px/1.4 ui-monospace, SFMono-Regular, Menlo, monospace;
  color: #94a3b8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tp-hook__status {
  font-size: 10px;
  font-weight: 700;
  color: #67e8f9;
}
.tp-hook__subject {
  padding: 10px 12px 0;
  font-size: 12px;
  font-weight: 600;
  color: #e2e8f0;
  overflow-wrap: anywhere;
}
</style>
