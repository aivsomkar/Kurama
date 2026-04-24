package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework AppKit -framework Foundation

#include <CoreGraphics/CoreGraphics.h>
#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>

void getMousePosition(double *x, double *y) {
    CGEventRef event = CGEventCreate(NULL);
    CGPoint point = CGEventGetLocation(event);
    CFRelease(event);
    *x = (double)point.x;
    *y = (double)point.y;
}

// Set the window origin in global screen coordinates (CG convention:
// origin is the top-left of the primary display, y increases downward).
//
// Ebitengine's SetWindowPosition is relative to the monitor the window is
// currently on, which causes the window to jump off-screen when crossing
// between monitors on an extended desktop. This bypasses that by setting
// the NSWindow frame origin directly in global coordinates.
void setWindowGlobalPosition(double cgX, double cgY, double windowHeight) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray<NSScreen *> *screens = [NSScreen screens];
        if (screens.count == 0) {
            return;
        }
        // AppKit's global coordinate space has its origin at the
        // bottom-left of the primary display. Convert from CG's
        // top-left-of-primary convention by flipping y against the
        // primary display's height.
        CGFloat primaryHeight = NSMaxY([screens[0] frame]);
        NSPoint topLeft = NSMakePoint(cgX, primaryHeight - cgY);
        for (NSWindow *window in [[NSApplication sharedApplication] windows]) {
            if (![window isVisible]) {
                continue;
            }
            [window setFrameTopLeftPoint:topLeft];
        }
    });
}

static void applyAllSpaces(void) {
    for (NSWindow *window in [[NSApplication sharedApplication] windows]) {
        [window setCollectionBehavior:
            NSWindowCollectionBehaviorCanJoinAllSpaces |
            NSWindowCollectionBehaviorStationary |
            NSWindowCollectionBehaviorIgnoresCycle |
            NSWindowCollectionBehaviorFullScreenAuxiliary];
        [window setLevel:NSScreenSaverWindowLevel + 1];
    }
}

// Called once from Go. Schedules an NSTimer on the main run loop so it
// fires reliably every 2 s regardless of GCD queue draining.
//
// Also switches the process activation policy to Accessory. Without this,
// the app is a regular foreground app, and macOS will hide its windows
// when another app enters full-screen mode — FullScreenAuxiliary only
// floats windows into full-screen Spaces for accessory/background apps.
void setWindowOnAllSpaces(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        applyAllSpaces();
        [NSTimer scheduledTimerWithTimeInterval:2.0
                                        repeats:YES
                                          block:^(NSTimer *t) {
            applyAllSpaces();
        }];
    });
}
*/
import "C"

func globalCursorPosition() (float64, float64) {
	var x, y C.double
	C.getMousePosition(&x, &y)
	return float64(x), float64(y)
}

func setWindowOnAllSpaces() {
	C.setWindowOnAllSpaces()
}

func setWindowGlobalPosition(x, y, windowHeight float64) {
	C.setWindowGlobalPosition(C.double(x), C.double(y), C.double(windowHeight))
}
