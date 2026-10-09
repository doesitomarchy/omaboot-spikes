OmaBoot? Live spike (look + graphics + llvmpipe), 2026-10-08

1. Boot the Mac from the Omarchy ISO stick (hold Option, pick "EFI Boot"). The GRUB menu shows
   for 15 s: pick "Omarchy" normally, or "Omarchy, nomodeset (safe graphics)" (key s) if the
   screen comes up distorted (the iMac10,1 needs it).
2. When the installer appears, press Ctrl+Alt+F2 (on Mac keyboards: Ctrl+Option+Fn+F2 if F2 alone
   does nothing). Log in as: root   (no password)
3. Plug in this stick, then type:
       mount -L BOOTLIVE /mnt && /mnt/run.sh
4. Follow the screen (about 5 minutes). q stops at any step; results so far are kept.
5. At the end:   sync; umount /mnt   then unplug this stick.
   Ctrl+Alt+F1 goes back to the installer; or just hold the power button.

Results: results/<Mac model>_<date-time>/ (result.json, egl.json, egl.log, kmsg-gpu.txt).
No serial numbers or disk IDs are recorded. Nothing is written to the Mac's own disks.

Patched drivers (second run, optional):
   mount -L BOOTLIVE /mnt && /mnt/run-modules.sh
Loads our fixed applesmc (fans, temperatures, light sensor, keyboard backlight) on any Mac,
and on the iMac10,1 (booted with the nomodeset entry) the radeon panel fix plus the GPU test.
Modules live in RAM only; a restart undoes them. Built for the Omarchy 4.0.4 ISO's kernel.
