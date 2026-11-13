<template>
  <div class="slide-captcha" :style="{ width: width + 'px', margin: '0 auto' }">
    <!-- Background image -->
    <div class="captcha-bg" :style="{ width: width + 'px', height: height + 'px' }">
      <canvas ref="bgCanvasRef" :width="width" :height="height" style="display: block" />
    </div>

    <!-- Sliding track -->
    <div
      ref="trackRef"
      class="captcha-track"
      :class="trackClass"
      @pointerdown="handlePointerDown"
      @pointermove="handlePointerMove"
      @pointerup="handlePointerUp"
      @pointercancel="handlePointerUp"
    >
      <!-- Passed area -->
      <div
        v-if="sliderLeft > 0"
        class="track-passed"
        :class="passedClass"
        :style="{ width: sliderLeft + PUZZLE_SIZE / 2 + 'px' }"
      />
      <!-- Hint text -->
      <div class="track-label" :class="labelClass">
        <template v-if="status === 'success'">
          <CheckOutlined /> {{ $t('captcha_verified') }}
        </template>
        <template v-else-if="status === 'fail'">
          <CloseOutlined /> {{ $t('captcha_verifyFailed') }}
        </template>
        <template v-else-if="status === 'sliding'">
          {{ $t('captcha_dragging') }}
        </template>
        <template v-else>
          {{ $t('captcha_dragHint') }}
        </template>
      </div>
      <!-- Slider -->
      <div
        class="slider-btn"
        :style="{ left: sliderLeft + 'px', backgroundColor: sliderBgColor }"
      >
        <!-- Icon -->
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <path v-if="status === 'success'" d="M5 10l3 3 7-7" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
          <template v-else>
            <path d="M6 6h2v8H6V6z" fill="#fff" />
            <path d="M12 6h2v8h-2V6z" fill="#fff" />
          </template>
        </svg>
        <!-- Puzzle piece preview -->
        <canvas
          ref="pieceCanvasRef"
          :width="PUZZLE_SIZE"
          :height="PUZZLE_SIZE"
          class="piece-preview"
          :style="{ opacity: status === 'idle' || status === 'sliding' ? 1 : 0 }"
        />
      </div>
    </div>

    <!-- Bottom refresh -->
    <div class="captcha-footer">
      <span class="refresh-link" @click="generate">
        <ReloadOutlined /> {{ $t('captcha_refresh') }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { ReloadOutlined, CheckOutlined, CloseOutlined } from '@ant-design/icons-vue'

const props = defineProps({
  width: { type: Number, default: 320 },
  height: { type: Number, default: 160 },
})

const emit = defineEmits(['success'])

const PUZZLE_SIZE = 48
const TOLERANCE = 5

const gapX = ref(0)
const sliderLeft = ref(0)
const status = ref('idle') // idle | sliding | success | fail
const dragging = ref(false)
const bgCanvasRef = ref(null)
const pieceCanvasRef = ref(null)
const trackRef = ref(null)

const puzzleY = (props.height - PUZZLE_SIZE) / 2

// Draw the background.
function drawBackground(ctx, w, h) {
  ctx.clearRect(0, 0, w, h)
  const bgGrad = ctx.createLinearGradient(0, 0, w, h)
  bgGrad.addColorStop(0, `hsl(${180 + Math.random() * 60}, 40%, 85%)`)
  bgGrad.addColorStop(1, `hsl(${190 + Math.random() * 40}, 35%, 75%)`)
  ctx.fillStyle = bgGrad
  ctx.fillRect(0, 0, w, h)

  for (let i = 0; i < 6; i++) {
    const x = Math.random() * w
    const y = Math.random() * h
    const size = 15 + Math.random() * 30
    ctx.fillStyle = `hsla(${180 + Math.random() * 60}, ${50 + Math.random() * 40}%, ${40 + Math.random() * 30}%, 0.6)`
    ctx.beginPath()
    if (Math.random() > 0.5) {
      ctx.arc(x, y, size / 2, 0, Math.PI * 2)
    } else {
      ctx.fillRect(x, y, size, size * (0.6 + Math.random() * 0.8))
    }
    ctx.fill()
  }

  ctx.fillStyle = 'rgba(100, 140, 180, 0.15)'
  ctx.font = 'bold 20px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText('GoTeams', w / 2, h / 2)
  ctx.fillText('DRAG', w / 2, h / 2 + 25)
}

// Combined path.
function drawPuzzlePath(ctx, x, y, size) {
  const r = 6
  const tabW = size * 0.28
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.lineTo(x + size * 0.35, y)
  ctx.arc(x + size * 0.35 + tabW / 2, y, tabW / 2, Math.PI, 0)
  ctx.lineTo(x + size * 0.65, y)
  ctx.arcTo(x + size, y, x + size, y + r, r)
  ctx.lineTo(x + size, y + size * 0.3)
  ctx.arc(x + size, y + size * 0.3 + tabW / 2, tabW / 2, -Math.PI / 2, Math.PI / 2)
  ctx.lineTo(x + size, y + size * 0.7)
  ctx.arcTo(x + size, y + size, x + size - r, y + size, r)
  ctx.lineTo(x + size * 0.65, y + size)
  ctx.arc(x + size * 0.65 - tabW / 2, y + size, tabW / 2, 0, Math.PI, true)
  ctx.lineTo(x + size * 0.35, y + size)
  ctx.arcTo(x, y + size, x, y + size - r, r)
  ctx.lineTo(x, y + r)
  ctx.arcTo(x, y, x + r, y, r)
  ctx.closePath()
}

// Cutout notch.
function drawCutout(ctx, x, y, size) {
  ctx.save()
  ctx.globalCompositeOperation = 'destination-out'
  drawPuzzlePath(ctx, x, y, size)
  ctx.fill()
  ctx.restore()

  ctx.save()
  ctx.strokeStyle = 'rgba(255,255,255,0.7)'
  ctx.lineWidth = 1.5
  ctx.setLineDash([4, 3])
  drawPuzzlePath(ctx, x, y, size)
  ctx.stroke()
  ctx.restore()
}

function generate() {
  const x = PUZZLE_SIZE + 10 + Math.random() * (props.width - PUZZLE_SIZE * 2 - 20)
  gapX.value = x
  sliderLeft.value = 0
  status.value = 'idle'

  nextTick(() => {
    const bgCanvas = bgCanvasRef.value
    const pieceCanvas = pieceCanvasRef.value
    if (!bgCanvas || !pieceCanvas) return
    const bgCtx = bgCanvas.getContext('2d')
    const pieceCtx = pieceCanvas.getContext('2d')
    if (!bgCtx || !pieceCtx) return

    drawBackground(bgCtx, props.width, props.height)
    drawCutout(bgCtx, x, puzzleY, PUZZLE_SIZE)

    drawBackground(pieceCtx, props.width, props.height)
    pieceCtx.save()
    drawPuzzlePath(pieceCtx, 0, puzzleY, PUZZLE_SIZE)
    pieceCtx.clip()
    pieceCtx.clearRect(0, 0, props.width, props.height)
    drawBackground(pieceCtx, props.width, props.height)
    pieceCtx.restore()

    const imgData = pieceCtx.getImageData(x, puzzleY, PUZZLE_SIZE, PUZZLE_SIZE)
    pieceCanvas.width = PUZZLE_SIZE
    pieceCanvas.height = PUZZLE_SIZE
    const newCtx = pieceCanvas.getContext('2d')
    newCtx.putImageData(imgData, 0, 0)
    newCtx.save()
    newCtx.globalCompositeOperation = 'destination-in'
    drawPuzzlePath(newCtx, 0, 0, PUZZLE_SIZE)
    newCtx.fill()
    newCtx.restore()

    newCtx.strokeStyle = 'rgba(255,255,255,0.8)'
    newCtx.lineWidth = 1.5
    drawPuzzlePath(newCtx, 0, 0, PUZZLE_SIZE)
    newCtx.stroke()
  })
}

onMounted(() => generate())

const sliderBgColor = computed(() => {
  if (status.value === 'success') return '#52c41a'
  if (status.value === 'fail') return '#ff4d4f'
  return '#0d68ff'
})

const trackClass = computed(() => ({
  'is-success': status.value === 'success',
  'is-fail': status.value === 'fail',
}))

const passedClass = computed(() => ({
  'is-success': status.value === 'success',
  'is-fail': status.value === 'fail',
}))

const labelClass = computed(() => ({
  'is-fail': status.value === 'fail',
  'is-success': status.value === 'success',
}))

function handlePointerDown(e) {
  if (status.value === 'success') return
  dragging.value = true
  e.target.setPointerCapture(e.pointerId)
}

function handlePointerMove(e) {
  if (!dragging.value || status.value === 'success') return
  const track = trackRef.value
  if (!track) return
  const rect = track.getBoundingClientRect()
  let x = e.clientX - rect.left - PUZZLE_SIZE / 2
  const maxLeft = props.width - PUZZLE_SIZE
  x = Math.max(0, Math.min(x, maxLeft))
  sliderLeft.value = x
  if (status.value === 'idle') status.value = 'sliding'
}

function handlePointerUp() {
  if (!dragging.value) return
  dragging.value = false
  if (status.value !== 'sliding') return

  if (Math.abs(sliderLeft.value - gapX.value) <= TOLERANCE) {
    status.value = 'success'
    sliderLeft.value = gapX.value
    setTimeout(() => emit('success'), 400)
  } else {
    status.value = 'fail'
    setTimeout(generate, 500)
  }
}
</script>

<style scoped lang="less">
.slide-captcha {
  user-select: none;
}

.captcha-bg {
  position: relative;
  border-radius: 6px;
  overflow: hidden;
  margin-bottom: 16px;
  border: 1px solid #e8e8e8;
}

.captcha-track {
  position: relative;
  width: 100%;
  height: 48px;
  border-radius: 24px;
  background-color: #f0f2f5;
  border: 1px solid #d9d9d9;
  overflow: hidden;
  cursor: grab;
  transition: border-color 0.3s;

  &.is-success { border-color: #52c41a; cursor: default; }
  &.is-fail { border-color: #ff4d4f; }
}

.track-passed {
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  background-color: rgba(22, 119, 255, 0.1);

  &.is-success { background-color: rgba(82, 196, 26, 0.15); transition: width 0.3s; }
  &.is-fail { background-color: rgba(255, 77, 79, 0.1); }
}

.track-label {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: #8c95a8;
  pointer-events: none;

  &.is-fail { color: #ff4d4f; }
  &.is-success { color: #52c41a; }
}

.slider-btn {
  position: absolute;
  top: -0.5px;
  width: v-bind('PUZZLE_SIZE + "px"');
  height: 48px;
  border-radius: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 2;
}

.piece-preview {
  position: absolute;
  bottom: 105%;
  left: 0;
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.4);
}

.captcha-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}

.refresh-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #0d68ff;
  cursor: pointer;
}
</style>
