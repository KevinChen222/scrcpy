# 应用图标

`icon.png` 是从用户提供的 Gemini 设计图中提取的透明背景主图；`icon.ico` 是同一图案的 Windows 多尺寸格式（16 / 24 / 32 / 48 / 64 / 128 / 256 px）。后续构建沿用这两个资源，替换设计时同步更新它们。

提取使用内置 imagegen；PNG 转 ICO 仅进行尺寸与格式转换。采用的提示词：

```text
Use case: background-extraction
Asset type: Windows desktop application icon, transparent PNG master.
Input image 1 is the edit target. Extract ONLY the complete white rounded-square app tile with its original centered overlapping teal green rounded phone rectangles and mint-green center. Remove all surrounding pale background, any external shadow and the bottom-right sparkle/watermark. Preserve the exact original symbol geometry, orientation, overlap/translucency, colors and white rounded-square plate; do not redesign or add anything. Crop and center the tile on a square transparent canvas so the tile nearly fills the canvas with a small even 3% margin. Crisp smooth edges, actual alpha transparency outside the rounded square, no text, no new shadow, no mockup scenery.
```
