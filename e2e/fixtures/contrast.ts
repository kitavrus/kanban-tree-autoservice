import type { Locator } from '@playwright/test';

/**
 * WCAG-контраст текста элемента к его ФАКТИЧЕСКОМУ фону (TS-046).
 *
 * Цвета берутся из computed style и приводятся к RGBA браузером (1×1 canvas), а не разбором
 * строки: computed-значение бывает не только `rgb(…)`/`rgba(…)`, но и `color(srgb 0.89 0.9
 * 0.91)` — так Chromium отдаёт результат `color-mix()` (им в app.css заданы фоны .alert-*),
 * где компоненты — доли 0…1, а не 0…255. Фон собирается наложением всех слоёв от корня к
 * элементу с учётом альфы (полупрозрачный фон смешивается с подложкой предка, подложка
 * страницы — фон body/html, иначе белый холст); полупрозрачный цвет текста накладывается на
 * итоговый фон.
 */
export async function contrastOf(el: Locator): Promise<number> {
  return el.evaluate((node) => {
    type RGBA = { r: number; g: number; b: number; a: number };
    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true })!;
    const toRGBA = (css: string): RGBA => {
      ctx.clearRect(0, 0, 1, 1);
      ctx.fillStyle = '#000';
      ctx.fillStyle = css; // нераспознанную строку canvas игнорирует — останется #000 (computed-цвета валидны всегда)
      ctx.fillRect(0, 0, 1, 1);
      const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
      return { r, g, b, a: a / 255 };
    };
    const over = (top: RGBA, bottom: RGBA): RGBA => ({
      r: top.r * top.a + bottom.r * (1 - top.a),
      g: top.g * top.a + bottom.g * (1 - top.a),
      b: top.b * top.a + bottom.b * (1 - top.a),
      a: 1,
    });
    const lum = (c: RGBA) => {
      const f = (v: number) => {
        const s = v / 255;
        return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
      };
      return 0.2126 * f(c.r) + 0.7152 * f(c.g) + 0.0722 * f(c.b);
    };

    // слои фона: от элемента вверх до <html>; накладываются снизу вверх на белый холст.
    const layers: RGBA[] = [];
    for (let n: Element | null = node; n; n = n.parentElement) {
      const style = getComputedStyle(n);
      if (style.backgroundImage !== 'none') throw new Error(`contrastOf: фон-картинка у <${n.tagName.toLowerCase()}> — контраст по цвету не определён`);
      layers.push(toRGBA(style.backgroundColor));
    }
    let bg: RGBA = { r: 255, g: 255, b: 255, a: 1 };
    for (const layer of layers.reverse()) bg = over(layer, bg);

    const fg = over(toRGBA(getComputedStyle(node).color), bg);
    const [hi, lo] = [lum(fg), lum(bg)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
  });
}
