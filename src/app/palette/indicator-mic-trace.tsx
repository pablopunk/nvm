import { useEffect, useRef } from 'react';

const SAMPLE_INTERVAL_MS = 100;
const VISIBLE_SAMPLES = 48;
const MIN_AMPLITUDE = 0.04;
const MAX_PIXEL_RATIO = 2;
const TRACE_HEIGHT_FRACTION = 0.9;
const LINE_WIDTH_HEIGHT_DIVISOR = 28;
const MIN_LINE_WIDTH = 1.5;
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
  const middle = height / HALF;
  context.clearRect(0, 0, width, height);
  context.strokeStyle = color;
  context.lineWidth = Math.max(
    MIN_LINE_WIDTH,
    height / LINE_WIDTH_HEIGHT_DIVISOR,
  );
  context.lineJoin = 'round';
  context.beginPath();
  samples.forEach((level, index) => {
    const direction = index % HALF === 0 ? 1 : -1;
    const amplitude =
      Math.max(MIN_AMPLITUDE, level) * middle * TRACE_HEIGHT_FRACTION;
    const x = (index - scrollFraction) * stepX;
    const y = middle - direction * amplitude;
    if (index === 0) {
      context.moveTo(x, y);
    } else {
      context.lineTo(x, y);
    }
  });
  context.stroke();
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
    let lastSampleAt = performance.now();
    let frame = 0;

    const unsubscribe = window.nvm.onIndicatorMicLevel((level) => {
      if (level == null) {
        return;
      }
      samples.push(level);
      samples.shift();
      lastSampleAt = performance.now();
    });

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
      cancelAnimationFrame(frame);
    };
  }, []);

  return <canvas ref={canvasRef} className="indicatorMicTrace" />;
}
