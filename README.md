# OmaBoot? spikes

Throwaway experiments for **OmaBoot? Live**, a bootable test suite that tells you how well
[Omarchy](https://omarchy.org) runs on an Intel Mac before you install it. Results feed
[DoesItOmarchy](https://doesitomarchy.com).

Each spike answers one risky question on real hardware before the real tool is built. The code
is meant to be thrown away; the answers are what carry forward.

| Spike | Question | Status |
|---|---|---|
| [`spike-look/`](spike-look/) | Do our console fonts, 16 colours and animations work on a Mac's text console, and can the live ISO use the GPU? | Done on 3 Macs, 2026-10-08 |
| [`iso-grub-patch/`](iso-grub-patch/) | How do you get a GRUB menu (and `nomodeset`) on the official ISO? | Done: used for every run above |

Still to come: a GRUB theme and Option-key startup icon on our own ISO build, the build and
test loop, an `applesmc` fix for pre-T2 Macs, and the iMac10,1 panel fix in a live boot.

MIT licensed, except the fonts in `spike-look/fonts/`, which keep their own licences.
