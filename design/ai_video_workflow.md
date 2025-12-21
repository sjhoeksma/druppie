# Technisch Ontwerp: AI Film Productie Pipeline

## 🎯 Doelstelling
Het automatiseren van het filmproductieproces door middel van Generative AI. Dit ontwerp beschrijft hoe we van een **script** (tekst) naar een **volledige video** gaan, gebruikmakend van **Headless ComfyUI** op Kubernetes voor schaalbare, GPU-intensieve generatie.

## 🏗️ Architectuur

De pipeline bestaat uit drie hoofdfasen die worden georkestreerd door de **Director Agent** (Orchestrator). De "Heavy Lifting" vindt plaats in de **K8s Render Farm**.

### Componenten
1.  **Director Agent (LLM)**: Vertaalt het verhaal naar technische prompts (Storyboard).
2.  **Scene Generator (ComfyUI API)**: Genereert losse clips op basis van prompts (HunyuanVideo).
3.  **Voice Generator (TTS API)**: Genereert gesproken Nederlandse tekst (XTTSv2 / Parkiet).
4.  **Editor (FFmpeg Worker)**: Voegt clips, overgangen en audio samen.

---

## 🎬 De Workflow

### Stap 1: Audio Eerst (Timing)
Voordat we beeld maken, genereert de **TTS Service** (XTTSv2/Parkiet) de volledige voice-over.
*   **Doel**: De lengte van de audio bepaalt de exacte lengte van de videosnede.
*   **Action**: `Text -> Audio (.wav)`.
*   **Result**: We weten nu: "Scene 1 duurt 4.2 seconden".

### Stap 2: Storyboard (Thumbnails)
We genereren voor elke scène **één statisch beeld** (Start Image). Dit is goedkoop en snel (seconden).
*   **Model**: Flux.1 of SDXL (via ComfyUI).
*   **Output**: `scene_01_thumb.png`.

### Stap 3: Animatic & Approval (Human-in-the-Loop)
De Agent maakt een preview (Animatic): de static images gemonteerd op de audio.
*   **User Action**: De gebruiker ziet het storyboard met geluid.
*   **Feedback**: "Scene 2 is te donker", "Tekst in Scene 1 loopt niet lekker".
*   **Kostenbesparing**: Mislukte ideeën worden hier gefixt *voordat* we dure video-GPU minuten verbranden.

### Stap 4: Video Productie (Hunyuan I2V)
Na "AKKOORD" start pas de zware renderfarm.
*   **Input**: De `scene_01_thumb.png` (als Image-to-Video input) + de duur van Stap 1.
*   **Model**: HunyuanVideo (Image-to-Video modus).
*   **Consistentie**: Omdat we een start-image gebruiken, "morph" de video exact vanuit het goedgekeurde plaatje.

### Stap 5: Final Montage
De **FFmpeg Worker** stikt de High-Res videoclips (`.mp4`) aan elkaar met de reeds goedgekeurde audio (`.wav`).

### Stap 6: Levering
1.  De eindfilm (`final_movie.mp4`) wordt beschikbaar gemaakt in de UI.
2.  De tijdelijke render-pods schalen automatisch af (Scale-to-Zero).

---

## 🛠️ Technische Specificatie (ComfyUI API Payload)

Voorbeeld payload voor **Stap 4** (Image-to-Video):

```json
{
  "client_id": "director_agent_007",
  "prompt": {
    "10": {
      "class_type": "LoadImage",
      "inputs": {
        "image": "scene_01_thumb.png" // Goedgekeurde Start Image
      }
    },
    "3": {
      "class_type": "HunyuanVideoSampler",
      "inputs": {
        "seed": 849302,
        "steps": 30,
        "frame_count": 125, // 5.0 seconden audio * 25 fps
        "fps": 25,
        "visual_condition": ["10", 0], // Link naar Start Image
        "text_condition": ["6", 0]
      }
    },
    "6": {
      "class_type": "CLIPTextEncode",
      "inputs": {
        "text": "Cyberpunk detective walking, rain..."
      }
    }
  }
}
```

## ✅ Voordelen
*   **Schaalbaar**: Het cluster verdeelt de scènes over alle beschikbare GPU's (Parallel Rendering).
*   **Automatisering**: Geen menselijke interactie nodig; van tekst tot video in één pijplijn.
*   **Gestandaardiseerd**: Door Docker containers is de output, in tegenstelling tot lokale machines, altijd identiek.
