"""Fetch attributed Commons media for the isolated visual-demo build."""
from concurrent.futures import ThreadPoolExecutor
import hashlib
import io
import json
import subprocess
import sys
from pathlib import Path
import urllib.parse
import urllib.request
from PIL import Image, ImageOps

ROOT = Path(__file__).resolve().parents[1]
TARGET = ROOT / 'apps/miniapp/src/demo/photos'
FILES = {
    'cat': 'Tabby cat with blue eyes-3336579.jpg',
    'dog': 'Dülmen, Hausdülmen, Golden Retriever -- 2022 -- 5530.jpg',
    'flowers': 'Garden daisies with insect, Ehrenbach.jpg',
    'ocean': 'Waves at La Corniche.jpg',
    'coffee': 'Gray espresso cup with amaretto.jpg',
    'books': 'Stack of books on table.jpg',
    'camera': 'Camera on stack of books (Unsplash).jpg',
    'keyboard': '2018 Bay Area Mechanical Keyboard Meetup (45222827204).jpg',
    'lamp': 'Retro desk lamp.jpg',
    'headphones': 'Bose QuietComfort 25 Acoustic Noise Cancelling Headphones with Carry Case.jpg',
    'library': "Duke Humfrey's Library Interior 1, Bodleian Library, Oxford, UK - Diliff.jpg",
    'sunset': 'Golden Hour Sunset Lake Georgetown TX 2024.jpg',
    'reading': 'Old books, desk lamp and computer.jpg',
    'bicycle': 'Parked bicycle with graffitied building facade and doors in Amsterdam.jpg',
    'lightsea': 'Btsthewingstour.jpg',
    'concert': 'Concert crowd (Unsplash).jpg',
    'lightsticks': 'Chemical-light01.jpg',
}

def download(url):
    result = subprocess.run(['curl.exe', '--fail', '--location', '--silent', '--show-error', '--retry', '2', '--max-time', '30', '--user-agent', 'CampusCircle-VisualDemo/1.0', url], capture_output=True)
    if result.returncode:
        raise RuntimeError('Photo request failed: ' + url.split('?')[0])
    return result.stdout

def prepare(item):
    key, title = item
    query = urllib.parse.urlencode({'action':'query','titles':'File:' + title,'prop':'imageinfo','iiprop':'url|extmetadata','iiurlwidth':800,'format':'json'})
    payload = json.loads(download('https://commons.wikimedia.org/w/api.php?' + query))
    page = next(iter(payload['query']['pages'].values()))
    info = page['imageinfo'][0]
    meta = info['extmetadata']
    license_name = meta['LicenseShortName']['value']
    if not license_name.startswith(('CC BY', 'CC0', 'Public domain')):
        raise RuntimeError('Unsupported license for ' + key)
    original = download(info.get('thumburl', info['url']))
    photo = ImageOps.exif_transpose(Image.open(io.BytesIO(original))).convert('RGB')
    photo.thumbnail((640, 640), Image.Resampling.LANCZOS)
    output = TARGET / (key + '.jpg')
    photo.save(output, quality=72, optimize=True, progressive=True)
    import re
    artist = re.sub('<[^>]+>', '', meta.get('Artist', {}).get('value', ''))
    return {'id':key,'title':title,'source':info['descriptionurl'],'artist':artist,
            'license':license_name,'licenseUrl':meta.get('LicenseUrl', {}).get('value', ''),
            'modification':'Resized and JPEG recompressed; rendered with object-fit crop. No AI generation.',
            'width':photo.width,'height':photo.height,'bytes':output.stat().st_size,
            'sha256':hashlib.sha256(output.read_bytes()).hexdigest()}

if __name__ == '__main__':
    TARGET.mkdir(parents=True, exist_ok=True)
    chosen = {key: title for key, title in FILES.items() if not sys.argv[1:] or key in sys.argv[1:]}
    with ThreadPoolExecutor(max_workers=3) as pool:
        updated = list(pool.map(prepare, chosen.items()))
    existing = TARGET.parent / 'media-sources.json'
    sources = json.loads(existing.read_text(encoding='utf-8')) if sys.argv[1:] and existing.exists() else []
    sources = [item for item in sources if item['id'] not in chosen] + updated
    sources.sort(key=lambda item: list(FILES).index(item['id']))
    (TARGET.parent / 'media-sources.json').write_text(json.dumps(sources, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(json.dumps({'photos':len(sources), 'bytes':sum(s['bytes'] for s in sources)}, ensure_ascii=False))
