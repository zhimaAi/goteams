<template>
  <div class="settings-panel priority-panel">
    <a-card :title="$t('settings_priorityList')" size="small" class="settings-card">
      <template #extra v-if="canManage">
        <a-space :size="4">
          <a-popconfirm
            :title="$t('settings_resetPriorityConfirm')"
            :ok-text="$t('common_confirm')"
            :cancel-text="$t('common_cancel')"
            @confirm="resetToDefault"
          >
            <a-button type="default" size="small" :loading="resetLoading" :disabled="defaultSaving">{{ $t('settings_restoreDefault') }}</a-button>
          </a-popconfirm>
          <a-button type="primary" size="small" @click="openEditor(null)"><PlusOutlined /> {{ $t('settings_addStatus') }}</a-button>
        </a-space>
      </template>
      <div class="enum-list">
        <div v-for="item in priorities" :key="item.id" class="enum-item">
          <span class="enum-tag" :style="{ color:item.color, borderColor:item.color }">{{ item.label }}</span>
          <a-space v-if="canManage" :size="8" class="row-actions">
            <a-button type="text" size="small" @click="openEditor(item)"><EditOutlined /></a-button>
            <a-button type="text" size="small" @click="deleteItem(item)"><DeleteOutlined /></a-button>
          </a-space>
        </div>
        <span v-if="!priorities.length" style="color:#999">{{ $t('settings_noPriority') }}</span>
      </div>
    </a-card>

    <a-card :title="$t('settings_defaultPriority')" size="small" class="settings-card">
      <a-radio-group
        :value="defaultPriorityId"
        class="default-priority-options"
        :disabled="!canManage || defaultSaving || resetLoading"
        @change="saveDefaultPriority"
      >
        <a-radio :value="0">{{ $t('settings_none') }}</a-radio>
        <a-radio v-for="item in priorities" :key="item.id" :value="Number(item.id)">
          <span class="enum-tag" :style="{ color:item.color, borderColor:item.color }">{{ item.label }}</span>
        </a-radio>
      </a-radio-group>
    </a-card>

    <a-modal v-model:open="modalOpen" :title="editingItem!==null ? $t('settings_editPriority') : $t('settings_addPriority')" @ok="handleSave" :ok-text="$t('common_save')" :cancel-text="$t('common_cancel')" width="440px" :destroy-on-close="true">
      <a-form layout="vertical" style="margin-top:12px">
        <a-form-item :label="$t('settings_priorityName')" required><a-input v-model:value="editLabel" :placeholder="$t('settings_priorityNamePlaceholder')" :maxlength="10" /></a-form-item>
        <a-form-item :label="$t('settings_color')">
          <div style="display:flex;gap:8px;flex-wrap:wrap">
            <div v-for="c in presetColors" :key="c" class="color-swatch" :class="{sel:editColor===c}" :style="{backgroundColor:c}" @click="editColor=c">
              <span v-if="editColor===c" :style="{color:isLight(c)?'#333':'#fff'}">✓</span>
            </div>
          </div>
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons-vue'
import { listPriorities, createPriority, updatePriority, updateDefaultPriority, deletePriority, resetPriorities } from '@/api/workspace'
import { useAuthStore } from '@/stores/auth'
import { typeSettingPermission } from '@/utils/typePermission'

const props = defineProps({
  wsId: { type: [String, Number], default: '' },
  typeKey: { type: String, default: 'requirement' },
})

const { t: $t } = useI18n()
const authStore = useAuthStore()
const presetColors = ['#ff4d4f','#fa8c16','#0d68ff','#8c8c8c','#52c41a','#722ed1','#eb2f96','#faad14','#13c2c2','#a0d911','#fa541c']

const priorities = ref([])
const modalOpen = ref(false)
const editingItem = ref(null)
const editLabel = ref('')
const editColor = ref('#fa8c16')
const resetLoading = ref(false)
const defaultPriorityId = ref(0)
const defaultSaving = ref(false)
const canManage = computed(() => authStore.hasPermission(typeSettingPermission(props.typeKey, 'priority')))

function isLight(h){ const r=parseInt(h.slice(1,3),16),g=parseInt(h.slice(3,5),16),b=parseInt(h.slice(5,7),16);return(0.299*r+0.587*g+0.114*b)/255>0.6 }

async function fetchPriorities(){
  const wsId=props.wsId
  const typeKey=props.typeKey
  try{
    const res=await listPriorities(wsId, typeKey)
    if(wsId!==props.wsId || typeKey!==props.typeKey)return
    priorities.value=res||[]
    defaultPriorityId.value=Number(priorities.value.find((item)=>Number(item.is_default)===1)?.id)||0
  }catch(e){ /* Business and network errors are handled centrally by the interceptor. */ }
}

async function saveDefaultPriority(event){
  if(!canManage.value || defaultSaving.value)return
  const wsId=props.wsId
  const typeKey=props.typeKey
  const previous=defaultPriorityId.value
  const next=Number(event.target.value)||0
  defaultPriorityId.value=next
  defaultSaving.value=true
  try{
    await updateDefaultPriority(wsId, typeKey, next)
    if(wsId!==props.wsId || typeKey!==props.typeKey)return
    priorities.value=priorities.value.map((item)=>({
      ...item,
      is_default:Number(item.id)===next ? 1 : 0,
    }))
    message.success($t('common_saved'))
  }catch(e){
    if(wsId===props.wsId && typeKey===props.typeKey)defaultPriorityId.value=previous
  }finally{
    defaultSaving.value=false
  }
}

function openEditor(item){
  if(!canManage.value)return
  if(item){ editLabel.value=item.label||''; editColor.value=item.color||'#fa8c16'; editingItem.value=item }
  else { editLabel.value=''; editColor.value='#fa8c16'; editingItem.value=null }
  modalOpen.value=true
}
async function handleSave(){
  if(!canManage.value)return
  if(!editLabel.value){message.warning($t('settings_nameRequired'));return}
  try{
    if(editingItem.value){
      await updatePriority(props.wsId, props.typeKey, editingItem.value.id, { name:editLabel.value, color:editColor.value, sort_order:editingItem.value.sort_order })
      message.success($t('settings_updated'))
    }else{
      await createPriority(props.wsId, props.typeKey, { name:editLabel.value, color:editColor.value, sort_order:priorities.value.length+1 })
      message.success($t('settings_added'))
    }
    modalOpen.value=false
    await fetchPriorities()
  }catch(e){}
}
async function deleteItem(item){
  if(!canManage.value)return
  try{
    await deletePriority(props.wsId, props.typeKey, item.id)
    message.success($t('settings_deleted'))
    await fetchPriorities()
  }catch(e){}
}

// Reset the current type's priorities to built-in defaults.
async function resetToDefault(){
  if(!canManage.value || defaultSaving.value)return
  resetLoading.value=true
  try{
    await resetPriorities(props.wsId, props.typeKey)
    message.success($t('settings_restored'))
    await fetchPriorities()
  }catch(e){}
  finally{ resetLoading.value=false }
}

onMounted(fetchPriorities)
watch(()=>[props.wsId, props.typeKey], ()=>{
  priorities.value=[]
  defaultPriorityId.value=0
  fetchPriorities()
})
</script>

<style scoped>
.settings-panel{width:min(920px, 100%);margin:0 auto}
.settings-card{margin-bottom:24px;overflow:hidden;border-color:#eef1f6;border-radius:12px;box-shadow:0 2px 8px rgba(24,43,80,.04)}
.settings-card :deep(.ant-card-head){min-height:54px;padding:0 16px;background:#f5f7fa;border-color:#eef1f6;font-weight:600}
.settings-card :deep(.ant-card-head-title){padding:15px 0;font-size:14px}
.settings-card :deep(.ant-card-body){padding:16px}
.enum-list{display:flex;flex-direction:column}
.enum-item{display:flex;align-items:center;justify-content:space-between;min-height:53px;padding:0 4px;border-bottom:1px solid #eef1f6}
.enum-item:last-child{border-bottom:0}
.enum-tag{display:inline-flex;align-items:center;height:22px;padding:0 10px;border:1px solid;border-radius:12px;font-size:12px;font-weight:600;line-height:22px;white-space:nowrap;background:transparent}
.default-priority-options{display:flex;align-items:center;flex-wrap:wrap;gap:20px 28px;min-height:28px}
.default-priority-options :deep(.ant-radio-wrapper){margin-inline-end:0}
.row-actions :deep(.ant-btn){color:#94a3b8;padding:0 6px}
.row-actions :deep(.ant-btn:hover){color:#3157e2;background:transparent}
.color-swatch{width:32px;height:32px;border-radius:6px;cursor:pointer;border:2px solid transparent;box-sizing:border-box;display:flex;align-items:center;justify-content:center;transition:border .15s;font-size:12px;font-weight:700}
.color-swatch.sel{border:3px solid #333}
</style>
