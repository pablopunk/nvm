import os from 'node:os';
import type {
  ExtensionMenuBar,
  NevermindExtension,
} from '../resources/nevermind-extension-api';

let previousCpuTimes = cpuTimes();

function cpuTimes() {
  return os.cpus().reduce(
    (total, cpu) => {
      const busy =
        cpu.times.user + cpu.times.nice + cpu.times.sys + cpu.times.irq;
      return { busy: total.busy + busy, idle: total.idle + cpu.times.idle };
    },
    { busy: 0, idle: 0 },
  );
}

function cpuPercent() {
  const current = cpuTimes();
  const busy = current.busy - previousCpuTimes.busy;
  const idle = current.idle - previousCpuTimes.idle;
  previousCpuTimes = current;
  return busy + idle > 0 ? Math.round((busy / (busy + idle)) * 100) : 0;
}

function memoryPercent() {
  return Math.round((1 - os.freemem() / os.totalmem()) * 100);
}

function meter(label: string, percent: number) {
  return `<div class="row"><span>${label}</span><b>${percent}%</b></div><div class="bar"><i style="width:${percent}%"></i></div>`;
}

function monitorHtml(cpu: number, memory: number) {
  return `<!doctype html><html><head><style>
body{margin:0;padding:14px;font-family:var(--nvm-font-sans);background:var(--nvm-surface-panel);color:var(--nvm-text-primary);border-radius:var(--nvm-radius-md)}
h1{font-size:13px;margin:0 0 10px;color:var(--nvm-text-secondary);font-weight:600}
.row{display:flex;justify-content:space-between;font-size:13px;margin-top:10px}
.bar{height:6px;border-radius:var(--nvm-radius-pill);background:var(--nvm-surface-sunken);margin-top:6px;overflow:hidden}
.bar i{display:block;height:100%;background:var(--nvm-accent-fill)}
</style></head><body><h1>${os.hostname()}</h1>${meter('CPU', cpu)}${meter('Memory', memory)}</body></html>`;
}

function monitorMenuBar(): ExtensionMenuBar {
  const cpu = cpuPercent();
  const memory = memoryPercent();
  return {
    icon: 'activity',
    title: `${cpu}%`,
    tooltip: 'System Monitor Fixture',
    popover: {
      width: 280,
      height: 150,
      view: {
        type: 'webview',
        title: 'System Monitor',
        html: monitorHtml(cpu, memory),
      },
    },
  };
}

const extension: NevermindExtension = {
  id: 'dev.ui-menubar-popover-fixture',
  title: 'Menu Bar Popover Fixture',
  subtitle: 'Dev-only Extension API menu bar popover fixture',
  menuBar: {
    triggers: [{ type: 'interval', every: '2s' }],
    render: monitorMenuBar,
  },
};

export default extension;
