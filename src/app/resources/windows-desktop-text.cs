using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Windows.Automation;

internal static class DesktopText {
  [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr window, out uint pid);
  [DllImport("user32.dll")] static extern bool SetForegroundWindow(IntPtr window);
  [DllImport("user32.dll")] static extern bool IsIconic(IntPtr window);
  [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr window, int command);
  [DllImport("user32.dll")] static extern IntPtr OpenInputDesktop(uint flags, bool inherit, uint access);
  [DllImport("user32.dll")] static extern bool CloseDesktop(IntPtr desktop);
  [DllImport("user32.dll")] static extern short GetAsyncKeyState(int key);
  [DllImport("user32.dll", SetLastError = true)] static extern uint SendInput(uint count, Input[] input, int size);
  [DllImport("kernel32.dll")] static extern IntPtr OpenProcess(uint access, bool inherit, uint pid);
  [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
  [DllImport("advapi32.dll")] static extern bool OpenProcessToken(IntPtr process, uint access, out IntPtr token);
  [DllImport("advapi32.dll")] static extern bool GetTokenInformation(IntPtr token, int kind, IntPtr data, int size, out int length);
  [DllImport("advapi32.dll")] static extern IntPtr GetSidSubAuthorityCount(IntPtr sid);
  [DllImport("advapi32.dll")] static extern IntPtr GetSidSubAuthority(IntPtr sid, uint index);

  [StructLayout(LayoutKind.Sequential)] struct KeyboardInput {
    public ushort key, scan;
    public uint flags, time;
    public UIntPtr extra;
  }
  [StructLayout(LayoutKind.Sequential)] struct MouseInput {
    public int x, y;
    public uint data, flags, time;
    public UIntPtr extra;
  }
  [StructLayout(LayoutKind.Explicit)] struct InputData {
    [FieldOffset(0)] public KeyboardInput keyboard;
    [FieldOffset(0)] public MouseInput mouse;
  }
  [StructLayout(LayoutKind.Sequential)] struct Input {
    public uint type;
    public InputData data;
  }

  static void Fail(string message, int code) {
    Console.Error.Write(message);
    Environment.Exit(code);
  }

  static uint ProcessId(IntPtr window) {
    uint pid;
    GetWindowThreadProcessId(window, out pid);
    return pid;
  }

  static int IntegrityLevel(uint pid) {
    IntPtr process = OpenProcess(0x1000, false, pid);
    IntPtr token = IntPtr.Zero;
    IntPtr data = IntPtr.Zero;
    try {
      if (process == IntPtr.Zero || !OpenProcessToken(process, 8, out token))
        Fail("Windows blocked access to the source app. Run that app without administrator rights.", 2);
      int length;
      GetTokenInformation(token, 25, IntPtr.Zero, 0, out length);
      data = Marshal.AllocHGlobal(length);
      if (!GetTokenInformation(token, 25, data, length, out length))
        Fail("Could not check Windows process permissions.", 5);
      IntPtr sid = Marshal.ReadIntPtr(data);
      byte count = Marshal.ReadByte(GetSidSubAuthorityCount(sid));
      return Marshal.ReadInt32(GetSidSubAuthority(sid, (uint)(count - 1)));
    } finally {
      if (data != IntPtr.Zero) Marshal.FreeHGlobal(data);
      if (token != IntPtr.Zero) CloseHandle(token);
      if (process != IntPtr.Zero) CloseHandle(process);
    }
  }

  static void CheckAccess(uint pid) {
    IntPtr desktop = OpenInputDesktop(0, false, 1);
    if (desktop == IntPtr.Zero) Fail("Windows blocked access to this desktop. Unlock your session and try again.", 2);
    CloseDesktop(desktop);
    if (pid != 0 && IntegrityLevel(pid) > IntegrityLevel((uint)Process.GetCurrentProcess().Id))
      Fail("Windows blocks access to elevated apps. Run the source app without administrator rights.", 2);
  }

  static void CheckTarget(IntPtr window, uint pid) {
    if (window == IntPtr.Zero || GetForegroundWindow() != window || ProcessId(window) != pid)
      Fail("The source window lost focus. Select the text and try again.", 4);
    CheckAccess(pid);
  }

  static void Focus(IntPtr window, uint pid) {
    if (ProcessId(window) != pid) Fail("The source window is no longer available.", 4);
    if (IsIconic(window)) ShowWindow(window, 9);
    SetForegroundWindow(window);
    for (int attempt = 0; attempt < 10; attempt++) {
      if (GetForegroundWindow() == window) return;
      Thread.Sleep(20);
    }
    Fail("Windows could not restore the source window. Select the text and try again.", 4);
  }

  static void Read(IntPtr window, uint pid) {
    CheckTarget(window, pid);
    AutomationElement element = AutomationElement.FocusedElement;
    if (element == null) Environment.Exit(3);
    if (element.Current.IsPassword) Fail("Protected text cannot be read.", 5);
    object pattern;
    if (!element.TryGetCurrentPattern(TextPattern.Pattern, out pattern)) Environment.Exit(3);
    TextPatternRange[] ranges = ((TextPattern)pattern).GetSelection();
    var text = new StringBuilder();
    foreach (TextPatternRange range in ranges) text.Append(range.GetText(-1));
    CheckTarget(window, pid);
    if (text.Length == 0) Environment.Exit(3);
    Console.Write(text.ToString());
  }

  static Input Key(ushort key, bool up) {
    return new Input { type = 1, data = new InputData {
      keyboard = new KeyboardInput { key = key, flags = up ? 2u : 0u }
    } };
  }

  static bool ModifiersHeld() {
    foreach (int key in new int[] { 16, 17, 18, 91, 92 })
      if ((GetAsyncKeyState(key) & 0x8000) != 0) return true;
    return false;
  }

  static void Shortcut(IntPtr window, uint pid, ushort key) {
    for (int attempt = 0; attempt < 30 && ModifiersHeld(); attempt++) Thread.Sleep(10);
    if (ModifiersHeld()) Fail("Release the shortcut keys and try again.", 5);
    CheckTarget(window, pid);
    Input[] keys = new Input[] { Key(17, false), Key(key, false), Key(key, true), Key(17, true) };
    if (SendInput((uint)keys.Length, keys, Marshal.SizeOf(typeof(Input))) != keys.Length)
      Fail("Windows blocked keyboard input. Check the source app permissions and try again.", 2);
  }

  [STAThread]
  static void Main(string[] args) {
    Console.OutputEncoding = new UTF8Encoding(false);
    try {
      if (args.Length == 1 && args[0] == "target") {
        IntPtr window = GetForegroundWindow();
        uint pid = ProcessId(window);
        if (window == IntPtr.Zero || pid == 0) Environment.Exit(3);
        Console.Write(pid + "\n" + window.ToInt64());
        return;
      }
      if (args.Length == 1 && args[0] == "permissions") {
        CheckAccess(ProcessId(GetForegroundWindow()));
        return;
      }
      uint targetPid;
      long handle;
      if (args.Length != 3 || !uint.TryParse(args[1], out targetPid) || targetPid == 0 || !long.TryParse(args[2], out handle) || handle <= 0)
        Fail("Invalid desktop-text target.", 5);
      targetPid = uint.Parse(args[1]);
      IntPtr target = new IntPtr(long.Parse(args[2]));
      switch (args[0]) {
        case "restore": Focus(target, targetPid); break;
        case "read": Read(target, targetPid); break;
        case "copy": Shortcut(target, targetPid, 67); break;
        case "paste": Shortcut(target, targetPid, 86); break;
        default: Fail("Unknown desktop-text operation.", 5); break;
      }
    } catch (UnauthorizedAccessException) {
      Fail("Windows blocked access to the source app. Run that app without administrator rights.", 2);
    } catch (ElementNotAvailableException) {
      Fail("The source window is no longer available.", 4);
    } catch {
      Fail("The Windows selected-text helper failed. Restart Nevermind and try again.", 5);
    }
  }
}
