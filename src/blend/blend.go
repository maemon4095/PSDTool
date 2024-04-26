//go:generate go run blend.go

package main

import (
	"log"
	"os"
	"strings"
	"text/template"
)

type code string

func (c code) Channel(ch string) string {
	return strings.NewReplacer("src", "s"+ch, "dest", "d"+ch, "ret", ch).Replace(string(c))
}

var blendBase = `
function blend{{.Name}}(d: Uint8ClampedArray, s: Uint8ClampedArray, w: number, h: number, alpha: number): void {
    let sr: number, sg: number, sb: number, sa: number, dr: number, dg: number, db: number, da: number;
    let a1: number, a2: number, a3: number, r: number, g: number, b: number, a: number, tmp: number;
    for (let i = 0, len = w * h << 2; i < len; i += 4) {
        sr = s[i];
        sg = s[i + 1];
        sb = s[i + 2];
        sa = s[i + 3];
        dr = d[i];
        dg = d[i + 1];
        db = d[i + 2];
        da = d[i + 3];

        tmp = 0 | (sa * alpha * 32897);
        a1 = (tmp * da) >> 23;
        a2 = (tmp * (255 - da)) >> 23;
        a3 = ((8388735 - tmp) * da) >> 23;
        a = a1 + a2 + a3;
        d[i + 3] = a;
        if (a) {
{{if .CodePerChannel}}{{.CodePerChannel.Channel "r"}}{{.CodePerChannel.Channel "g"}}{{.CodePerChannel.Channel "b"}}{{else}}{{.Code}}{{end}}
            d[i] = (r * a1 + sr * a2 + dr * a3) / a;
            d[i + 1] = (g * a1 + sg * a2 + dg * a3) / a;
            d[i + 2] = (b * a1 + sb * a2 + db * a3) / a;
        }
    }
}
`

var blendModes = []struct {
	Name           string
	Code           string
	CodePerChannel code
}{
	// ----------------------------------------------------------------
	// References:
	// https://www.w3.org/TR/compositing-1/#blending
	// http://dunnbypaul.net/blends/
	// https://mouaif.wordpress.com/2009/01/05/photoshop-math-with-glsl-shaders/
	// ----------------------------------------------------------------
	{
		Name: "Normal",
		CodePerChannel: `
            ret = src;
`,
	},
	// ----------------------------------------------------------------
	{
		Name: "Darken",
		CodePerChannel: `
            if (src < dest) {
                ret = src;
            } else {
                ret = dest;
            }
`,
	},
	{
		Name: "Multiply",
		CodePerChannel: `
            ret = src * dest * 32897 >> 23;
`,
	},
	{
		Name: "ColorBurn",
		CodePerChannel: `
            if (dest === 255) {
                ret = 255;
            } else if (src === 0) {
                ret = 0;
            } else {
                ret = 255 - Math.min(255, (255 - dest) / src * 255);
            }
`,
	},
	{
		Name: "LinearBurn",
		CodePerChannel: `
            ret = Math.max(0, dest + src - 255);
`,
	},
	{
		Name: "DarkerColor",
		Code: `
            if (lum(sr, sg, sb) < lum(dr, dg, db)) {
                r = sr;
                g = sg;
                b = sb;
            } else {
                r = dr;
                g = dg;
                b = db;
            }
`,
	},
	// ----------------------------------------------------------------
	{
		Name: "Lighten",
		CodePerChannel: `
            if (src > dest) {
                ret = src;
            } else {
                ret = dest;
            }
`,
	},
	{
		Name: "Screen",
		CodePerChannel: `
            ret = src + dest - (src * dest * 32897 >> 23);
`,
	},
	{
		Name: "ColorDodge",
		CodePerChannel: `
            if (dest === 0) {
                ret = 0;
            } else if (src === 255) {
                ret = 255;
            } else {
                ret = Math.min(255, dest * 255 / (255 - src));
            }
`,
	},
	{
		Name: "LinearDodge",
		CodePerChannel: `
            ret = src + dest;
`,
	},
	{
		Name: "LighterColor",
		Code: `
            if (lum(sr, sg, sb) > lum(dr, dg, db)) {
                r = sr;
                g = sg;
                b = sb;
            } else {
                r = dr;
                g = dg;
                b = db;
            }
`,
	},
	// ----------------------------------------------------------------
	{
		Name: "Overlay",
		CodePerChannel: `
            if (dest < 128) {
                ret = src * dest * 32897 >> 22;
            } else {
                ret = 255 - ((255 - ((dest - 128) << 1)) * (255 - src) * 32897 >> 23);
            }
`,
	},
	{
		Name: "SoftLight",
		CodePerChannel: `
            if (src < 128) {
                ret = dest - (((255 - (src << 1)) * dest * 32897 >> 23) * (255 - dest) * 32897 >> 23);
            } else {
                if (dest < 64) {
                    tmp = ((((dest << 4) - 3060) * 32897 >> 23) * dest + 1020) * dest * 32897 >> 23;
                } else {
                    tmp = Math.sqrt(dest / 255) * 255;
                }
                ret = dest + (((src << 1) - 255) * (tmp - dest) * 32897 >> 23);
            }
`,
	},
	{
		Name: "HardLight",
		CodePerChannel: `
            if (src < 128) {
                ret = dest * src * 32897 >> 22;
            } else {
                tmp = (src << 1) - 255;
                ret = dest + tmp - (dest * tmp * 32897 >> 23);
            }
`,
	},
	{
		Name: "VividLight",
		CodePerChannel: `
            if (src < 128) {
                tmp = src << 1;
                if (src === 0) {
                    ret = tmp;
                } else {
                    ret = Math.max(0, (255 - ((255 - dest) * 255) / tmp));
                }
            } else {
                tmp = ((src - 128) << 1) + 1;
                /* if (dest === 0) {
                    ret = 255;
                } else */
                if (tmp === 255) {
                    ret = tmp;
                } else {
                    ret = Math.min(255, ((dest * 255) / (255 - tmp)));
                }
            }
`,
	},
	{
		Name: "LinearLight",
		CodePerChannel: `
            if (src < 128) {
                ret = dest + (src << 1) - 255;
            } else {
                ret = dest + ((src - 128) << 1);
            }
`,
	},
	{
		Name: "PinLight",
		CodePerChannel: `
            if (src < 128) {
                tmp = src << 1;
                if (tmp < dest) {
                    ret = tmp;
                } else {
                    ret = dest;
                }
            } else {
                tmp = (src - 128) << 1;
                if (tmp > dest) {
                    ret = tmp;
                } else {
                    ret = dest;
                }
            }
`,
	},
	{
		Name: "HardMix",
		CodePerChannel: `
            if (src < 128) {
                tmp = src << 1;
                if (src !== 0) {
                    tmp = Math.max(0, (255 - ((255 - dest) * 255) / tmp));
                }
            } else {
                if (dest === 0) {
                    tmp = 0;
                } else {
                    tmp = ((src - 128) << 1) + 1;
                    if (tmp !== 255) {
                        tmp = Math.min(255, ((dest * 255) / (255 - tmp)));
                    }
                }
            }
            ret = tmp < 128 ? 0 : 255;
`,
	},
	// ----------------------------------------------------------------
	{
		Name: "Difference",
		CodePerChannel: `
            tmp = dest - src;
            ret = tmp < 0 ? -tmp : tmp;
`,
	},
	{
		Name: "Exclusion",
		CodePerChannel: `
            ret = dest + src - (dest * src * 32897 >> 22);
`,
	},
	{
		Name: "Subtract",
		CodePerChannel: `
            ret = Math.max(0, dest - src);
`,
	},
	{
		Name: "Divide",
		CodePerChannel: `
            if (dest === 0) {
                ret = 0;
            } else if (src === 0) {
                ret = 255;
            } else {
                ret = Math.min(255, dest / src * 255);
            }
`,
	},
	// ----------------------------------------------------------------
	{
		Name: "Hue",
		Code: `
            tmp = setSat(sr, sg, sb, sat(dr, dg, db));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
            tmp = setLum(r, g, b, lum(dr, dg, db));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
`,
	},
	{
		Name: "Saturation",
		Code: `
            tmp = setSat(dr, dg, db, sat(sr, sg, sb));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
            tmp = setLum(r, g, b, lum(dr, dg, db));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
`,
	},
	{
		Name: "Color",
		Code: `
            tmp = setLum(sr, sg, sb, lum(dr, dg, db));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
`,
	},
	{
		Name: "Luminosity",
		Code: `
            tmp = setLum(dr, dg, db, lum(sr, sg, sb));
            r = (tmp & 0xff0000) >> 16;
            g = (tmp & 0xff00) >> 8;
            b = tmp & 0xff;
`,
	},
}

var source = `// DO NOT EDIT.
// Generate with: go generate
function lum(r: number, g: number, b: number): number {
    return (r * 77 + g * 151 + b * 28) >> 8;
}

function setLum(r: number, g: number, b: number, lm: number): number {
    lm -= lum(r, g, b);
    return clipColor(r + lm, g + lm, b + lm);
}

function clipColor(r: number, g: number, b: number): number {
    const lm = lum(r, g, b);
    const min = Math.min(r, g, b);
    const max = Math.max(r, g, b);
    if (min < 0) {
        r = lm + (((r - lm) * lm) / (lm - min));
        g = lm + (((g - lm) * lm) / (lm - min));
        b = lm + (((b - lm) * lm) / (lm - min));
    }
    if (max > 255) {
        r = lm + (((r - lm) * (255 - lm)) / (max - lm));
        g = lm + (((g - lm) * (255 - lm)) / (max - lm));
        b = lm + (((b - lm) * (255 - lm)) / (max - lm));
    }
    return (r << 16) | (g << 8) | b;
}

function sat(r: number, g: number, b: number): number {
    return Math.max(r, g, b) - Math.min(r, g, b);
}

function setSat(r: number, g: number, b: number, sat: number): number {
    if (r <= g) {
        if (g <= b) {
            return setSatMinMidMax(r, g, b, sat);
        } else if (r <= b) {
            sat = setSatMinMidMax(r, b, g, sat);
            return (sat & 0xff0000) | ((sat & 0xff) << 8) | ((sat & 0xff00) >> 8);
        }
        sat = setSatMinMidMax(b, r, g, sat);
        return ((sat & 0xffff) << 8) | ((sat & 0xff0000) >> 16);
    } else if (r <= b) {
        sat = setSatMinMidMax(g, r, b, sat);
        return ((sat & 0xff00) << 8) | ((sat & 0xff0000) >> 8) | (sat & 0xff);
    } else if (g <= b) {
        sat = setSatMinMidMax(g, b, r, sat);
        return ((sat & 0xff) << 16) | ((sat & 0xffff00) >> 8);
    }
    sat = setSatMinMidMax(b, g, r, sat);
    return ((sat & 0xff) << 16) | (sat & 0xff00) | ((sat & 0xff0000) >> 16);
}

function setSatMinMidMax(min: number, mid: number, max: number, sat: number): number {
    if (max > min) {
        return ((((mid - min) * sat) / (max - min)) << 8) | sat;
    }
    return 0;
}

function copyAlpha(d: Uint8ClampedArray, s: Uint8ClampedArray, w: number, h: number, alpha: number): void {
    for (let i = 0, len = w * h << 2; i < len; i += 4) {
        d[i + 3] = s[i + 3] * alpha;
    }
}

function copyOpaque(d: Uint8ClampedArray, s: Uint8ClampedArray, w: number, h: number, alpha: number): void {
    const a = 255 * alpha;
    for (let i = 0, len = w * h << 2; i < len; i += 4) {
        d[i + 0] = s[i + 0];
        d[i + 1] = s[i + 1];
        d[i + 2] = s[i + 2];
        d[i + 3] = a;
    }
}

{{range .}}{{template "blendBase" .}}{{end}}
const blendModes: {
    [b: string]: (d: Uint8ClampedArray, s: Uint8ClampedArray, w: number, h: number, alpha: number) => void;
} = {
    'copy-alpha': copyAlpha,
    'copy-opaque': copyOpaque,

    // 'pass-through': blendPassThrough,
    'source-over': blendNormal,
    // 'dissolve': blendDissolve,

    'darken': blendDarken,
    'multiply': blendMultiply,
    'color-burn': blendColorBurn,
    'linear-burn': blendLinearBurn,
    'darker-color': blendDarkerColor,

    'lighten': blendLighten,
    'screen': blendScreen,
    'color-dodge': blendColorDodge,
    'linear-dodge': blendLinearDodge,
    'lighter-color': blendLighterColor,

    'overlay': blendOverlay,
    'soft-light': blendSoftLight,
    'hard-light': blendHardLight,
    'vivid-light': blendVividLight,
    'linear-light': blendLinearLight,
    'pin-light': blendPinLight,
    'hard-mix': blendHardMix,

    'difference': blendDifference,
    'exclusion': blendExclusion,
    'subtract': blendSubtract,
    'divide': blendDivide,

    'hue': blendHue,
    'saturation': blendSaturation,
    'color': blendColor,
    'luminosity': blendLuminosity
};

const implementedBlendModes = enumImplementedBlendModes();

export function blend(
    dest: CanvasRenderingContext2D,
    src: CanvasRenderingContext2D,
    sx: number,
    sy: number,
    dx: number,
    dy: number,
    w: number,
    h: number,
    alpha: number,
    blendMode: string
): void {
    if (blendMode === 'normal') {
        blendMode = 'source-over';
    }
    if (blendMode in implementedBlendModes) {
        dest.save();
        dest.globalAlpha = alpha;
        dest.globalCompositeOperation = blendMode as unknown as GlobalCompositeOperation;
        dest.drawImage(src.canvas, sx, sy, w, h, dx, dy, w, h);
        dest.restore();
        // console.log('native: '+blendMode);
        return;
    }

    if (dx < 0) {
        w -= dx;
        sx -= dx;
        dx = 0;
    }
    if (sx < 0) {
        w -= sx;
        dx -= sx;
        sx = 0;
    }
    if (dy < 0) {
        h -= dy;
        sy -= dy;
        dy = 0;
    }
    if (sy < 0) {
        h -= sy;
        dy -= sy;
        sy = 0;
    }
    w = Math.min(w, src.canvas.width - sx, dest.canvas.width - dx);
    h = Math.min(h, src.canvas.height - sy, dest.canvas.height - dy);
    if (w <= 0 || h <= 0 || alpha === 0) {
        return;
    }
    const imgData = dest.getImageData(dx, dy, w, h);
    const d = imgData.data;
    const s = src.getImageData(sx, sy, w, h).data;
    if (!(blendMode in blendModes)) {
        throw new Error('unimplemeneted blend mode: ' + blendMode);
    }
    blendModes[blendMode](d, s, w, h, alpha);
    dest.putImageData(imgData, dx, dy);
    // console.log('js: '+blendMode);
}

function enumImplementedBlendModes(): { [b: string]: undefined } {
    const r: { [b: string]: undefined } = {};
    const c = document.createElement('canvas');
    const ctx = c.getContext('2d');
    if (!ctx) {
        throw new Error('cannot get CanvasRenderingContext2D');
    }
    for (const bm of Object.keys(blendModes)) {
        ctx.globalCompositeOperation = bm as unknown as GlobalCompositeOperation;
        if (ctx.globalCompositeOperation === bm) {
            r[bm] = undefined;
        }
    }
    return r;
}

function detectBrokenColorDodge(): Promise<boolean> {
    return new Promise(resolve => {
        const img = new Image();
        img.src = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg' +
            '0kAAAAGUlEQVQI1wXBAQEAAAgCIOz/5TJI20UGhz5D2wX8PWbkFQAAAABJRU5ErkJggg==';
        img.onload = e => {
            const c = document.createElement('canvas');
            c.width = 257;
            c.height = 256;

            const ctx = c.getContext('2d');
            if (!ctx) {
                throw new Error('cannot get CanvasRenderingContext2D');
            }
            ctx.fillStyle = 'rgb(255, 255, 255)';
            ctx.fillRect(0, 0, c.width, c.height);
            ctx.globalAlpha = 0.5;
            ctx.globalCompositeOperation = 'color-dodge';
            ctx.drawImage(img, 0, 0);

            const d = ctx.getImageData(0, 0, 1, 1);
            resolve(d.data[0] < 128);
        };
    });
}

detectBrokenColorDodge().then(isBroken => {
    if (isBroken) {
        delete implementedBlendModes['color-dodge'];
    }
});
`

func main() {
	f, err := os.Create("blend.gen.ts")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	tpl := template.Must(template.New("").Parse(source))
	tpl.New("blendBase").Parse(blendBase)
	tpl.Execute(f, blendModes)
}
