import { FRAGMENT, VERTEX } from './shaders';

export function animateLightning(canvas: HTMLCanvasElement) {
  const gl = canvas.getContext('webgl2', { alpha: true, antialias: false });
  if (!gl) return;
  const program = gl.createProgram();
  for (const [type, source] of [
    [gl.VERTEX_SHADER, VERTEX],
    [gl.FRAGMENT_SHADER, FRAGMENT],
  ] as const) {
    const shader = gl.createShader(type);
    if (!shader) return;
    gl.shaderSource(shader, source);
    gl.compileShader(shader);
    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      gl.deleteShader(shader);
      gl.deleteProgram(program);
      return;
    }
    gl.attachShader(program, shader);
    gl.deleteShader(shader);
  }
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    gl.deleteProgram(program);
    return;
  }
  const buffer = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(
    gl.ARRAY_BUFFER,
    new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]),
    gl.STATIC_DRAW,
  );
  gl.useProgram(program);
  const position = gl.getAttribLocation(program, 'aPosition');
  gl.enableVertexAttribArray(position);
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
  function uniform(name: string) {
    return gl!.getUniformLocation(program, name);
  }
  const defaults = {
    uIntensity: 0.3,
    uSize: 1.2,
    uThickness: 0.75,
    uGlow: 0.65,
    uFlicker: 0.45,
    uBranches: 0.65,
    uAngle: 0.12,
    uSpread: 0.6,
    uFlash: 0,
    uLight: 0,
    uFade: 0.7,
  };
  for (const [name, value] of Object.entries(defaults))
    gl.uniform1f(uniform(name), value);
  gl.uniform1i(uniform('uBolts'), 1);
  gl.uniform3f(uniform('uColor'), 0.96, 0.69, 0.28);
  gl.uniform4f(uniform('uPointer'), 0, 0, 0, 0);
  gl.uniform4fv(uniform('uStrikes'), new Float32Array(16));
  gl.uniform4fv(uniform('uStrikeInfo'), new Float32Array(16));
  const resolution = uniform('uResolution');
  const phase = uniform('uPhase');
  const opacity = uniform('uOpacity');
  const intro = uniform('uIntro');
  const offset = uniform('uXOffset');
  const bolts = uniform('uBolts');
  const angle = uniform('uAngle');
  const strikes = uniform('uStrikes');
  const strikeInfo = uniform('uStrikeInfo');
  const strikeData = new Float32Array(16);
  const infoData = new Float32Array(16);
  const speed = 3;
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');
  const video = document.querySelector('.landing-demo');
  function updateBounds() {
    const top = Math.max(
      0,
      Math.min(innerHeight, video?.getBoundingClientRect().bottom ?? 0),
    );
    canvas.style.setProperty('--lightning-top', `${top}px`);
  }
  function resizeCanvas() {
    const width = Math.max(1, canvas.clientWidth);
    const height = Math.max(1, canvas.clientHeight);
    const ratio = Math.min(1, 1280 / width, 900 / height);
    const targetWidth = Math.round(width * ratio);
    const targetHeight = Math.round(height * ratio);
    if (canvas.width !== targetWidth || canvas.height !== targetHeight) {
      canvas.width = targetWidth;
      canvas.height = targetHeight;
    }
    gl!.viewport(0, 0, canvas.width, canvas.height);
    gl!.uniform2f(resolution, canvas.width, canvas.height);
  }
  updateBounds();
  window.addEventListener('scroll', updateBounds, { passive: true });
  window.addEventListener('resize', updateBounds);

  let timer: ReturnType<typeof setTimeout>;
  let frame = 0;
  let first = true;
  let start = 0;
  let seed = 0;
  let clicked: { x: number; y: number; reach: number } | null = null;
  function clear() {
    clearTimeout(timer);
    cancelAnimationFrame(frame);
    gl!.clearColor(0, 0, 0, 0);
    gl!.clear(gl!.COLOR_BUFFER_BIT);
  }
  function draw(now: number) {
    const age = (now - start) / 1000;
    if (age > 0.65) {
      clear();
      schedule();
      return;
    }
    resizeCanvas();
    gl!.uniform1f(phase, seed + (age * speed) / 10);
    gl!.uniform1f(intro, Math.min(1, age / 0.06));
    gl!.uniform1f(
      opacity,
      Math.min(1, age / 0.035) *
        Math.max(0, Math.min(1, (0.65 - age) / 0.25)) *
        0.75,
    );
    strikeData.fill(0);
    infoData.fill(0);
    if (clicked) {
      const elapsed = age * 2;
      const progress = Math.min(1, elapsed / 0.11);
      const since = Math.max(0, elapsed - 0.11);
      const strength =
        elapsed < 0.11
          ? 0.45 + 0.35 * progress
          : 1.8 * Math.exp(-since * 5.5) * (0.85 + 0.2 * Math.sin(since * 70)) +
            (since > 0.17 ? Math.exp(-(since - 0.17) * 9) : 0);
      strikeData.set([
        clicked.x,
        clicked.y,
        clicked.reach * (1 - progress ** 1.6),
        strength,
      ]);
      infoData.set([seed * 100, clicked.reach, elapsed >= 0.11 ? 1 : 0, 0]);
    }
    gl!.uniform1i(bolts, clicked ? 0 : 1);
    gl!.uniform4fv(strikes, strikeData);
    gl!.uniform4fv(strikeInfo, infoData);
    gl!.drawArrays(gl!.TRIANGLE_STRIP, 0, 4);
    frame = requestAnimationFrame(draw);
  }
  function randomizeAngle() {
    const radians = ((Math.random() * 100 - 50) * Math.PI) / 180;
    gl!.uniform1f(angle, radians);
    return radians;
  }
  function strike() {
    clicked = null;
    randomizeAngle();
    if (canvas.clientHeight < 1) {
      schedule();
      return;
    }
    resizeCanvas();
    gl!.uniform1f(
      offset,
      (Math.random() - 0.5) *
        (canvas.clientWidth / Math.max(1, canvas.clientHeight)) *
        1.3,
    );
    seed = Math.random();
    start = performance.now();
    first = false;
    frame = requestAnimationFrame(draw);
  }
  function schedule() {
    clear();
    if (document.hidden || reducedMotion.matches || gl!.isContextLost()) return;
    timer = setTimeout(strike, first ? 1200 : 3000 + Math.random() * 5000);
  }
  function clickStrike(event: PointerEvent) {
    if (
      event.button !== 0 ||
      document.hidden ||
      reducedMotion.matches ||
      gl!.isContextLost()
    )
      return;
    updateBounds();
    const rect = canvas.getBoundingClientRect();
    if (
      rect.height < 1 ||
      event.clientY < rect.top ||
      event.clientY > rect.bottom
    )
      return;
    clear();
    resizeCanvas();
    const x = (2 * (event.clientX - rect.left) - rect.width) / rect.height;
    const y = (rect.height - 2 * (event.clientY - rect.top)) / rect.height;
    const radians = randomizeAngle();
    const sine = Math.sin(radians);
    const cosine = Math.cos(radians);
    clicked = {
      x,
      y,
      reach:
        (rect.width / rect.height) * Math.abs(sine) +
        Math.abs(cosine) -
        (-sine * x + cosine * y) +
        0.2,
    };
    seed = Math.random();
    start = performance.now();
    first = false;
    frame = requestAnimationFrame(draw);
  }
  window.addEventListener('pointerdown', clickStrike, { passive: true });
  document.addEventListener('visibilitychange', schedule);
  reducedMotion.addEventListener('change', schedule);
  canvas.addEventListener('webglcontextlost', clear);

  window.addEventListener(
    'pagehide',
    function dispose() {
      clear();
      document.removeEventListener('visibilitychange', schedule);
      reducedMotion.removeEventListener('change', schedule);
      window.removeEventListener('pointerdown', clickStrike);
      window.removeEventListener('scroll', updateBounds);
      window.removeEventListener('resize', updateBounds);
      gl!.deleteBuffer(buffer);
      gl!.deleteProgram(program);
    },
    { once: true },
  );
  schedule();
}
