<script setup lang="ts">
import { RsButton, RsDrawer, RsEmpty, RsInput, RsTree, type RsTreeNode } from '@/ui'

defineProps<{
  moduleId: string
  title: string
  treeData: RsTreeNode[]
}>()

const open = defineModel<boolean>('open', { required: true })
const keyword = defineModel<string>('keyword', { required: true })
const checkedKeys = defineModel<string[]>('checkedKeys', { required: true })
const expandedKeys = defineModel<string[]>('expandedKeys', { required: true })

const emit = defineEmits<{ save: [] }>()
</script>

<template>
  <RsDrawer v-model:open="open" :title="title" side="right" size="md" :teleport-to="false">
    <RsInput v-model="keyword" placeholder="搜索角色" clearable />
    <div class="fg-user-tree">
      <RsTree
        v-if="treeData.length"
        :nodes="treeData"
        v-model:checked-keys="checkedKeys"
        v-model:expanded-keys="expandedKeys"
        :filter="keyword"
        checkable
        check-strictly
        :selectable="false"
        show-line
        block-node
      />
      <RsEmpty v-else description="没有可分配的角色" />
    </div>
    <template #footer>
      <RsButton size="sm" @click="open = false">关闭</RsButton>
      <RsButton variant="primary" size="sm" @click="emit('save')">保存授权</RsButton>
    </template>
  </RsDrawer>
</template>

<style scoped>
.fg-user-tree {
  margin-top: 12px;
  min-height: 240px;
}
</style>
