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
    uIntensity: 0.85,
    uSize: 1.2,
    uThickness: 0.75,
    uGlow: 0.65,
    uFlicker: 0.45,
    uBranches: 0.65,
    uAngle: 0.12,
    uSpread: 0.6,
    uFlash: 0,
    uLight: 0,
    uFade: 0.2,
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
  const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');
  let timer: ReturnType<typeof setTimeout>;
  let frame = 0;
  let first = true;
  let start = 0;
  let seed = 0;
  function clear() {
    clearTimeout(timer);
    cancelAnimationFrame(frame);
    gl!.clearColor(0, 0, 0, 0);
    gl!.clear(gl!.COLOR_BUFFER_BIT);
  }
  function draw(now: number) {
    const age = (now - start) / 1000;
    if (age > 1.8) {
      clear();
      schedule();
      return;
    }
    gl!.uniform1f(phase, seed + age * 0.13);
    gl!.uniform1f(intro, Math.min(1, age / 0.15));
    gl!.uniform1f(
      opacity,
      Math.min(1, age / 0.08) *
        Math.max(0, Math.min(1, (1.8 - age) / 0.65)) *
        0.75,
    );
    gl!.drawArrays(gl!.TRIANGLE_STRIP, 0, 4);
    frame = requestAnimationFrame(draw);
  }
  function strike() {
    const ratio = Math.min(1, 1280 / innerWidth, 900 / innerHeight);
    canvas.width = Math.round(innerWidth * ratio);
    canvas.height = Math.round(innerHeight * ratio);
    gl!.viewport(0, 0, canvas.width, canvas.height);
    gl!.uniform2f(resolution, canvas.width, canvas.height);
    gl!.uniform1f(
      offset,
      (Math.random() - 0.5) * (innerWidth / innerHeight) * 1.3,
    );
    seed = Math.random();
    start = performance.now();
    first = false;
    frame = requestAnimationFrame(draw);
  }
  function schedule() {
    clear();
    if (document.hidden || reducedMotion.matches || gl!.isContextLost()) return;
    timer = setTimeout(strike, first ? 1200 : 7000 + Math.random() * 9000);
  }
  document.addEventListener('visibilitychange', schedule);
  reducedMotion.addEventListener('change', schedule);
  canvas.addEventListener('webglcontextlost', clear);

  window.addEventListener(
    'pagehide',
    function dispose() {
      clear();
      document.removeEventListener('visibilitychange', schedule);
      reducedMotion.removeEventListener('change', schedule);
      gl!.deleteBuffer(buffer);
      gl!.deleteProgram(program);
    },
    { once: true },
  );
  schedule();
}
