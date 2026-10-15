import { useEffect, useRef } from 'react';

const SAMPLE_INTERVAL_MS = 100;
const VISIBLE_SAMPLES = 48;
const MIN_AMPLITUDE = 0.06;
const MAX_PIXEL_RATIO = 2;
const TRACE_HEIGHT_FRACTION = 0.9;
const MIN_BAR_WIDTH = 1;
const BAR_WIDTH_FRACTION = 0.3;
const HALF = 2;

function traceColor(canvas: HTMLCanvasElement) {
  return getComputedStyle(canvas).getPropertyValue('--accent').trim() || '#888';
}

function drawTrace(
  canvas: HTMLCanvasElement,
  samples: number[],
  scrollFraction: number,
  color: string,
) {
  const context = canvas.getContext('2d');
  if (!context) {
    return;
  }
  const { width, height } = canvas;
  const stepX = width / (VISIBLE_SAMPLES - 1);
  const barWidth = Math.max(MIN_BAR_WIDTH, stepX * BAR_WIDTH_FRACTION);
  const middle = height / HALF;
  context.clearRect(0, 0, width, height);
  context.fillStyle = color;
  samples.forEach((level, index) => {
    const barHeight =
      Math.max(MIN_AMPLITUDE, level) * height * TRACE_HEIGHT_FRACTION;
    const x = (index - scrollFraction) * stepX - barWidth / HALF;
    context.fillRect(x, middle - barHeight / HALF, barWidth, barHeight);
  });
}

export function IndicatorMicTrace() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const ratio = Math.min(MAX_PIXEL_RATIO, window.devicePixelRatio || 1);
    canvas.width = canvas.clientWidth * ratio;
    canvas.height = canvas.clientHeight * ratio;
    const color = traceColor(canvas);
    const samples: number[] = new Array(VISIBLE_SAMPLES + 1).fill(0);
    let latestLevel = 0;
    let lastSampleAt = performance.now();
    let frame = 0;

    const unsubscribe = window.nvm.onIndicatorMicLevel((level) => {
      latestLevel = level ?? 0;
    });

    const sampleTimer = window.setInterval(() => {
      samples.push(latestLevel);
      samples.shift();
      lastSampleAt = performance.now();
    }, SAMPLE_INTERVAL_MS);

    const render = () => {
      const fraction = Math.min(
        1,
        (performance.now() - lastSampleAt) / SAMPLE_INTERVAL_MS,
      );
      drawTrace(canvas, samples, fraction, color);
      frame = requestAnimationFrame(render);
    };
    frame = requestAnimationFrame(render);

    return () => {
      unsubscribe();
      window.clearInterval(sampleTimer);
      cancelAnimationFrame(frame);
    };
  }, []);

  return <canvas ref={canvasRef} className="indicatorMicTrace" />;
}
