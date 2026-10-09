// egl-probe: open each DRM render node through GBM + EGL (no window system), try GLES 3, GLES 2
// and desktop GL contexts, and for GLES draw one shaded triangle into an offscreen texture and
// read a pixel back, then time a compositor-like 1080p frame. Prints one JSON object per node.
// "--software" probes Mesa's surfaceless platform instead (run it with LIBGL_ALWAYS_SOFTWARE=1:
// llvmpipe, no GPU needed). Built on the laptop, run on the live ISO against Mesa unpacked into
// RAM by egl.sh.
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <fcntl.h>
#include <gbm.h>
#include <stdio.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

static void jstr(const char *k, const char *v, int comma) {
    printf("\"%s\":\"", k);
    for (const char *p = v ? v : ""; *p; p++) {
        if (*p == '"' || *p == '\\') putchar('\\');
        if ((unsigned char)*p >= 0x20) putchar(*p);
    }
    printf("\"%s", comma ? "," : "");
}

static double now_ms(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return t.tv_sec * 1e3 + t.tv_nsec / 1e6;
}

static const char *VS = "attribute vec2 p; void main(){ gl_Position = vec4(p, 0.0, 1.0); }";
static const char *FS = "precision mediump float; void main(){ gl_FragColor = vec4(0.25, 0.5, 0.75, 1.0); }";

// Draw a full-screen triangle into a 64x64 texture; return 1 if the centre pixel is our colour.
static int draw_test(char *got, size_t gotlen, double *ms) {
    double t0 = now_ms();
    GLint ok = 0;
    GLuint vs = glCreateShader(GL_VERTEX_SHADER), fs = glCreateShader(GL_FRAGMENT_SHADER);
    glShaderSource(vs, 1, &VS, NULL); glCompileShader(vs); glGetShaderiv(vs, GL_COMPILE_STATUS, &ok);
    if (!ok) { snprintf(got, gotlen, "vertex shader did not compile"); return 0; }
    glShaderSource(fs, 1, &FS, NULL); glCompileShader(fs); glGetShaderiv(fs, GL_COMPILE_STATUS, &ok);
    if (!ok) { snprintf(got, gotlen, "fragment shader did not compile"); return 0; }
    GLuint pr = glCreateProgram();
    glAttachShader(pr, vs); glAttachShader(pr, fs); glBindAttribLocation(pr, 0, "p"); glLinkProgram(pr);
    glGetProgramiv(pr, GL_LINK_STATUS, &ok);
    if (!ok) { snprintf(got, gotlen, "program did not link"); return 0; }
    GLuint tex, fb;
    glGenTextures(1, &tex); glBindTexture(GL_TEXTURE_2D, tex);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, 64, 64, 0, GL_RGBA, GL_UNSIGNED_BYTE, NULL);
    glGenFramebuffers(1, &fb); glBindFramebuffer(GL_FRAMEBUFFER, fb);
    glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, tex, 0);
    if (glCheckFramebufferStatus(GL_FRAMEBUFFER) != GL_FRAMEBUFFER_COMPLETE) { snprintf(got, gotlen, "framebuffer incomplete"); return 0; }
    static const GLfloat tri[] = {-1, -1, 3, -1, -1, 3};
    glViewport(0, 0, 64, 64); glClearColor(0, 0, 0, 1); glClear(GL_COLOR_BUFFER_BIT);
    glUseProgram(pr); glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, 0, tri); glEnableVertexAttribArray(0);
    glDrawArrays(GL_TRIANGLES, 0, 3); glFinish();
    unsigned char px[4] = {0};
    glReadPixels(32, 32, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, px);
    *ms = now_ms() - t0;
    snprintf(got, gotlen, "%d,%d,%d,%d", px[0], px[1], px[2], px[3]);
    int d0 = px[0] - 64, d1 = px[1] - 128, d2 = px[2] - 191;
    return d0 * d0 < 9 && d1 * d1 < 9 && d2 * d2 < 9 && px[3] == 255;
}

static const char *BVS = "attribute vec2 p; varying vec2 uv; uniform vec4 r;"
    "void main(){ uv = p * 0.5 + 0.5; gl_Position = vec4(r.xy + p * r.zw, 0.0, 1.0); }";
static const char *BFS = "precision mediump float; varying vec2 uv; uniform sampler2D t;"
    "void main(){ vec4 c = texture2D(t, uv); gl_FragColor = vec4(c.rgb * 0.9 + 0.05, 0.92); }";

// Compositor-like load: a 1920x1080 wallpaper plus two blended, textured windows per frame,
// into a 1920x1080 offscreen target. Runs for about 2 s (at most 120 frames).
static void bench_1080(double *ms_frame, int *frames) {
    enum { W = 1920, H = 1080 };
    static unsigned char px[W * H * 4];
    for (int i = 0; i < W * H * 4; i++) px[i] = (unsigned char)(i * 7 + (i >> 11));
    GLint ok = 0;
    GLuint vs = glCreateShader(GL_VERTEX_SHADER), fs = glCreateShader(GL_FRAGMENT_SHADER);
    glShaderSource(vs, 1, &BVS, NULL); glCompileShader(vs);
    glShaderSource(fs, 1, &BFS, NULL); glCompileShader(fs);
    GLuint pr = glCreateProgram();
    glAttachShader(pr, vs); glAttachShader(pr, fs); glBindAttribLocation(pr, 0, "p"); glLinkProgram(pr);
    glGetProgramiv(pr, GL_LINK_STATUS, &ok);
    *ms_frame = -1; *frames = 0;
    if (!ok) return;
    GLuint tex[2], fb;
    glGenTextures(2, tex);
    for (int k = 0; k < 2; k++) {
        glBindTexture(GL_TEXTURE_2D, tex[k]);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
        glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA, W, H, 0, GL_RGBA, GL_UNSIGNED_BYTE, k ? px : NULL);
    }
    glGenFramebuffers(1, &fb); glBindFramebuffer(GL_FRAMEBUFFER, fb);
    glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, tex[0], 0);
    if (glCheckFramebufferStatus(GL_FRAMEBUFFER) != GL_FRAMEBUFFER_COMPLETE) return;
    static const GLfloat quad[] = {-1, -1, 1, -1, -1, 1, 1, 1};
    glViewport(0, 0, W, H); glUseProgram(pr);
    glBindTexture(GL_TEXTURE_2D, tex[1]);
    glUniform1i(glGetUniformLocation(pr, "t"), 0);
    GLint r = glGetUniformLocation(pr, "r");
    glVertexAttribPointer(0, 2, GL_FLOAT, GL_FALSE, 0, quad); glEnableVertexAttribArray(0);
    glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
    glFinish();
    double t0 = now_ms();
    int n = 0;
    while (n < 120 && now_ms() - t0 < 2000) {
        glDisable(GL_BLEND); glUniform4f(r, 0, 0, 1, 1); glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
        glEnable(GL_BLEND);
        glUniform4f(r, -0.3f, 0.1f, 0.55f, 0.6f); glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
        glUniform4f(r, 0.35f, -0.2f, 0.5f, 0.55f); glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
        glFinish();
        n++;
    }
    *frames = n;
    *ms_frame = (now_ms() - t0) / n;
}

static void try_api(EGLDisplay dpy, const char *name, EGLenum api, EGLint renderable, int major, int last) {
    printf("{");
    jstr("api", name, 1);
    EGLint cfgattr[] = {EGL_SURFACE_TYPE, 0, EGL_RENDERABLE_TYPE, renderable, EGL_NONE}; // any surface type: surfaceless has pbuffer configs only
    EGLConfig cfg;
    EGLint n = 0;
    if (!eglBindAPI(api) || !eglChooseConfig(dpy, cfgattr, &cfg, 1, &n) || n < 1) {
        printf("\"ok\":false,\"error\":\"no config (0x%x)\"}%s", eglGetError(), last ? "" : ",");
        return;
    }
    EGLint ctxattr[] = {EGL_CONTEXT_MAJOR_VERSION, major, EGL_NONE};
    EGLContext ctx = eglCreateContext(dpy, cfg, EGL_NO_CONTEXT, api == EGL_OPENGL_API ? NULL : ctxattr);
    if (ctx == EGL_NO_CONTEXT || !eglMakeCurrent(dpy, EGL_NO_SURFACE, EGL_NO_SURFACE, ctx)) {
        printf("\"ok\":false,\"error\":\"no context (0x%x)\"}%s", eglGetError(), last ? "" : ",");
        if (ctx != EGL_NO_CONTEXT) eglDestroyContext(dpy, ctx);
        return;
    }
    printf("\"ok\":true,");
    jstr("renderer", (const char *)glGetString(GL_RENDERER), 1);
    jstr("version", (const char *)glGetString(GL_VERSION), 1);
    jstr("glsl", (const char *)glGetString(GL_SHADING_LANGUAGE_VERSION), api == EGL_OPENGL_ES_API);
    if (api == EGL_OPENGL_ES_API) {
        char got[64];
        double ms = 0;
        int drawn = draw_test(got, sizeof got, &ms);
        printf("\"draw_ok\":%s,", drawn ? "true" : "false");
        jstr("draw_pixel", got, 1);
        printf("\"draw_ms\":%.1f", ms);
        if (major == 2) {
            double mf; int nf;
            bench_1080(&mf, &nf);
            printf(",\"bench_1080p_ms\":%.1f,\"bench_1080p_fps\":%.1f,\"bench_frames\":%d", mf, mf > 0 ? 1000 / mf : 0, nf);
        }
    }
    printf("}%s", last ? "" : ",");
    eglMakeCurrent(dpy, EGL_NO_SURFACE, EGL_NO_SURFACE, EGL_NO_CONTEXT);
    eglDestroyContext(dpy, ctx);
}

static void probe_display(EGLDisplay dpy) {
    EGLint maj = 0, min = 0;
    if (dpy == EGL_NO_DISPLAY || !eglInitialize(dpy, &maj, &min)) {
        printf("\"ok\":false,\"error\":\"eglInitialize failed (0x%x)\"}\n", eglGetError());
        return;
    }
    const char *ext = eglQueryString(dpy, EGL_EXTENSIONS);
    printf("\"ok\":true,\"egl\":\"%d.%d\",", maj, min);
    jstr("egl_vendor", eglQueryString(dpy, EGL_VENDOR), 1);
    printf("\"surfaceless\":%s,", ext && strstr(ext, "EGL_KHR_surfaceless_context") ? "true" : "false");
    printf("\"contexts\":[");
    try_api(dpy, "gles3", EGL_OPENGL_ES_API, EGL_OPENGL_ES3_BIT, 3, 0);
    try_api(dpy, "gles2", EGL_OPENGL_ES_API, EGL_OPENGL_ES2_BIT, 2, 0);
    try_api(dpy, "gl", EGL_OPENGL_API, EGL_OPENGL_BIT, 0, 1);
    printf("]}\n");
    fflush(stdout);
    eglTerminate(dpy);
}

int main(int argc, char **argv) {
    for (int i = 1; i < argc; i++) {
        printf("{");
        if (strcmp(argv[i], "--software") == 0) {
            jstr("node", "software", 1);
            probe_display(eglGetPlatformDisplay(EGL_PLATFORM_SURFACELESS_MESA, EGL_DEFAULT_DISPLAY, NULL));
            continue;
        }
        jstr("node", argv[i], 1);
        int fd = open(argv[i], O_RDWR | O_CLOEXEC);
        if (fd < 0) { printf("\"ok\":false,\"error\":\"open failed\"}\n"); continue; }
        struct gbm_device *gbm = gbm_create_device(fd);
        if (!gbm) { printf("\"ok\":false,\"error\":\"gbm_create_device failed\"}\n"); close(fd); continue; }
        jstr("gbm_backend", gbm_device_get_backend_name(gbm), 1);
        probe_display(eglGetPlatformDisplay(EGL_PLATFORM_GBM_KHR, gbm, NULL));
        gbm_device_destroy(gbm);
        close(fd);
    }
    return 0;
}
