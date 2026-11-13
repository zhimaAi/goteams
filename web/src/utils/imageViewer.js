// 图片预览：用 Viewer.js 的 viewer-canvas 覆盖层替代 ant-design-vue 的图片预览组。
// 预览层按需创建、关闭即销毁，避免常驻隐藏图片列表带来的额外 DOM 与图片请求。
import Viewer from "viewerjs";
import "viewerjs/dist/viewer.css";

let activeViewer = null;
let activeContainer = null;

// destroy() 内部会把 destroyed 置为 true，因此 hidden 回调里再次 destroy 不会递归。
function destroyViewer(viewer, container) {
  viewer.destroy();
  container.remove();
  if (activeViewer === viewer) {
    activeViewer = null;
    activeContainer = null;
  }
}

export function openImageViewer({ sources, index = 0 }) {
  const list = Array.isArray(sources) ? sources.filter(Boolean) : [];
  if (!list.length) return;
  closeImageViewer();
  // Viewer.js 需要一个承载 img 的元素来读取图片地址，容器本身不参与展示。
  const container = document.createElement("div");
  container.style.display = "none";
  container.setAttribute("aria-hidden", "true");
  list.forEach((src) => {
    const image = document.createElement("img");
    image.src = src;
    container.appendChild(image);
  });
  document.body.appendChild(container);
  // 构造阶段 Viewer 还未统计图片数量，此时 view(index) 会被越界判断拦掉，
  // 因此用 initialViewIndex 指定起始图片，再由 show() 完成构建与展示。
  const startIndex = Math.min(Math.max(index, 0), list.length - 1);
  const viewer = new Viewer(container, {
    initialViewIndex: startIndex,
    // Viewer.js 传入 toolbar 对象时只渲染列出的按钮，这里去掉幻灯片播放（play）。
    toolbar: {
      zoomIn: 1,
      zoomOut: 1,
      oneToOne: 1,
      reset: 1,
      prev: 1,
      next: 1,
      rotateLeft: 1,
      rotateRight: 1,
      flipHorizontal: 1,
      flipVertical: 1,
    },
    navbar: list.length > 1,
    keyboard: true,
    // Viewer.js 默认在每次 move 时给图片加回 .viewer-transition（all .3s），
    // 图片会用 0.3s 缓动追赶鼠标，拖动明显发滞；只对拖动关闭，其余动作保留动画。
    transition: { move: false },
    hidden: () => destroyViewer(viewer, container),
  });
  activeViewer = viewer;
  activeContainer = container;
  viewer.show();
}

export function closeImageViewer() {
  if (!activeViewer) return;
  destroyViewer(activeViewer, activeContainer);
}
