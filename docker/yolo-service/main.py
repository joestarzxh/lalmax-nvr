import os
import io
import base64
import time
from typing import List, Optional, Dict
from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from PIL import Image
import numpy as np
import cv2
import supervision as sv

# Global model instance
model = None
# Per-camera trackers for object tracking
trackers: Dict[str, sv.ByteTrack] = {}

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Load model on startup
    global model
    from ultralytics import YOLO
    model_name = os.getenv("MODEL", "yolov8n.pt")
    print(f"Loading YOLO model: {model_name}")
    model = YOLO(model_name)
    print("YOLO model loaded successfully")
    yield
    # Cleanup on shutdown
    model = None

app = FastAPI(
    title="YOLO Detection Service",
    description="YOLO object detection service for lalmax-nvr",
    version="1.0.0",
    lifespan=lifespan
)

# Request/Response models
class DetectionRequest(BaseModel):
    frame: str  # base64-encoded image
    camera_id: Optional[str] = None
    timestamp: Optional[int] = None
    confidence: Optional[float] = None  # Override default confidence

class Detection(BaseModel):
    label: str
    confidence: float
    box: List[float]  # [x, y, width, height] normalized
    track_id: Optional[int] = None  # Object tracking ID


def get_tracker(camera_id: str) -> sv.ByteTrack:
    """Get or create a ByteTrack tracker for a specific camera."""
    if camera_id not in trackers:
        trackers[camera_id] = sv.ByteTrack(
            track_activation_threshold=0.25,
            lost_track_buffer=30,
            minimum_matching_threshold=0.8,
            frame_rate=30
        )
    return trackers[camera_id]

class DetectionResponse(BaseModel):
    detections: List[Detection]
    processing_time_ms: float
    error: Optional[str] = None

class HealthResponse(BaseModel):
    status: str
    model: str
    device: str

# Health check endpoint
@app.get("/health", response_model=HealthResponse)
async def health_check():
    return HealthResponse(
        status="healthy" if model is not None else "loading",
        model=os.getenv("MODEL", "yolov8n.pt"),
        device="cpu"  # TODO: detect GPU
    )

# Detection endpoint
@app.post("/api/detect", response_model=DetectionResponse)
async def detect(request: DetectionRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")

    start_time = time.time()

    try:
        image_data = base64.b64decode(request.frame)
        img_array = decode_image(image_data)

        # Get confidence threshold
        conf_threshold = request.confidence or float(os.getenv("CONFIDENCE", "0.5"))

        # Run detection
        results = model(img_array, conf=conf_threshold, verbose=False)

        # Check if tracking is enabled
        enable_tracking = os.getenv("ENABLE_TRACKING", "true").lower() == "true"
        camera_id = request.camera_id or "default"
        img_h, img_w = img_array.shape[:2]

        # Parse results and apply tracking
        detections = []
        for result in results:
            boxes = result.boxes
            if boxes is not None and len(boxes) > 0:
                # Convert to supervision Detections format for tracking
                if enable_tracking:
                    sv_detections = sv.Detections.from_ultralytics(result)
                    tracker = get_tracker(camera_id)
                    sv_detections = tracker.update_with_detections(sv_detections)

                    # Extract tracked detections
                    for i in range(len(sv_detections)):
                        x1, y1, x2, y2 = sv_detections.xyxy[i]
                        class_id = sv_detections.class_id[i] if sv_detections.class_id is not None else 0
                        confidence = sv_detections.confidence[i] if sv_detections.confidence is not None else 0.0
                        track_id = sv_detections.tracker_id[i] if sv_detections.tracker_id is not None else None

                        # Normalize to [0, 1]
                        x_norm = float(x1) / img_w
                        y_norm = float(y1) / img_h
                        w_norm = float(x2 - x1) / img_w
                        h_norm = float(y2 - y1) / img_h

                        label = model.names[int(class_id)]

                        detections.append(Detection(
                            label=label,
                            confidence=float(confidence),
                            box=[x_norm, y_norm, w_norm, h_norm],
                            track_id=int(track_id) if track_id is not None else None
                        ))
                else:
                    # No tracking, just return raw detections
                    for box in boxes:
                        x1, y1, x2, y2 = box.xyxy[0].tolist()

                        # Normalize to [0, 1]
                        x_norm = x1 / img_w
                        y_norm = y1 / img_h
                        w_norm = (x2 - x1) / img_w
                        h_norm = (y2 - y1) / img_h

                        label = model.names[int(box.cls[0])]

                        detections.append(Detection(
                            label=label,
                            confidence=float(box.conf[0]),
                            box=[x_norm, y_norm, w_norm, h_norm]
                        ))

        processing_time = (time.time() - start_time) * 1000

        return DetectionResponse(
            detections=detections,
            processing_time_ms=round(processing_time, 2)
        )

    except Exception as e:
        processing_time = (time.time() - start_time) * 1000
        return DetectionResponse(
            detections=[],
            processing_time_ms=round(processing_time, 2),
            error=str(e)
        )

def decode_image(image_data: bytes) -> np.ndarray:
    """Decode a JPEG/PNG/WebP frame with OpenCV, falling back to PIL."""
    np_buf = np.frombuffer(image_data, dtype=np.uint8)
    img = cv2.imdecode(np_buf, cv2.IMREAD_COLOR)
    if img is not None:
        return cv2.cvtColor(img, cv2.COLOR_BGR2RGB)

    try:
        image = Image.open(io.BytesIO(image_data)).convert("RGB")
        return np.array(image)
    except Exception as exc:
        raise ValueError(
            "frame is not a decodable image; configure the NVR to send JPEG snapshots "
            "or add video-frame extraction before YOLO"
        ) from exc

# List available models
@app.get("/api/models")
async def list_models():
    return {
        "models": [
            {"name": "yolov8n.pt", "description": "YOLOv8 Nano - Fastest, least accurate"},
            {"name": "yolov8s.pt", "description": "YOLOv8 Small - Balanced"},
            {"name": "yolov8m.pt", "description": "YOLOv8 Medium - More accurate"},
            {"name": "yolov8l.pt", "description": "YOLOv8 Large - High accuracy"},
            {"name": "yolov8x.pt", "description": "YOLOv8 Extra Large - Most accurate"},
        ]
    }

# Get supported labels
@app.get("/api/labels")
async def get_labels():
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")
    return {"labels": list(model.names.values())}

if __name__ == "__main__":
    port = int(os.getenv("PORT", "8080"))
    uvicorn.run(app, host="0.0.0.0", port=port)
