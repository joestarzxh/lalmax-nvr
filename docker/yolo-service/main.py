"""
YOLO Detection Service for lalmax-nvr

A lightweight HTTP service that performs object detection using YOLOv8/v11.
Designed to be used as an external AI backend for lalmax-nvr.

Usage:
    python main.py

Environment Variables:
    PORT: Server port (default: 8080)
    MODEL: YOLO model name (default: yolov8n.pt)
    CONFIDENCE: Default confidence threshold (default: 0.5)
    DEVICE: Inference device (default: cpu, options: cpu, mps, cuda)
"""

import io
import os
import time
import base64
import logging
from typing import List, Optional

import numpy as np
from PIL import Image
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from ultralytics import YOLO

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("yolo-service")

# Configuration from environment
PORT = int(os.getenv("PORT", "8080"))
MODEL_NAME = os.getenv("MODEL", "yolov8n.pt")
DEFAULT_CONFIDENCE = float(os.getenv("CONFIDENCE", "0.5"))
DEVICE = os.getenv("DEVICE", "cpu")

# Initialize FastAPI app
app = FastAPI(
    title="YOLO Detection Service",
    description="Object detection service for lalmax-nvr",
    version="1.0.0",
)

# Global model instance
model: Optional[YOLO] = None


# Request/Response models
class Detection(BaseModel):
    label: str
    confidence: float
    box: List[float]  # [x, y, width, height] normalized


class DetectRequest(BaseModel):
    frame: str  # base64-encoded image
    camera_id: str = ""
    timestamp: int = 0
    confidence: Optional[float] = None


class DetectResponse(BaseModel):
    detections: List[Detection]
    inference_time_ms: float
    error: Optional[str] = None


class HealthResponse(BaseModel):
    status: str
    model: str
    device: str


@app.on_event("startup")
async def startup_event():
    """Load YOLO model on startup."""
    global model
    logger.info(f"Loading YOLO model: {MODEL_NAME} on device: {DEVICE}")
    try:
        model = YOLO(MODEL_NAME)
        # Warmup with a dummy image
        dummy_img = np.zeros((640, 640, 3), dtype=np.uint8)
        model.predict(dummy_img, device=DEVICE, verbose=False)
        logger.info(f"Model loaded successfully: {MODEL_NAME}")
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        raise


@app.get("/health", response_model=HealthResponse)
async def health():
    """Health check endpoint."""
    return HealthResponse(
        status="ok" if model else "error",
        model=MODEL_NAME,
        device=DEVICE,
    )


@app.post("/api/detect", response_model=DetectResponse)
async def detect(request: DetectRequest):
    """
    Perform object detection on a base64-encoded image.
    
    This endpoint is compatible with lalmax-nvr's HTTP AI backend.
    """
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded")

    start_time = time.time()

    try:
        # Decode base64 image
        image_bytes = base64.b64decode(request.frame)
        image = Image.open(io.BytesIO(image_bytes))

        # Get confidence threshold
        confidence = request.confidence or DEFAULT_CONFIDENCE

        # Run inference
        results = model.predict(
            image,
            conf=confidence,
            device=DEVICE,
            verbose=False,
        )

        # Parse results
        detections = []
        for result in results:
            boxes = result.boxes
            if boxes is None:
                continue

            for i in range(len(boxes)):
                box = boxes[i]
                # Get normalized coordinates (x_center, y_center, width, height)
                x_center, y_center, width, height = box.xywhn[0].tolist()
                confidence_score = float(box.conf[0])
                class_id = int(box.cls[0])
                label = result.names[class_id]

                detections.append(Detection(
                    label=label,
                    confidence=confidence_score,
                    box=[x_center, y_center, width, height],
                ))

        inference_time = (time.time() - start_time) * 1000

        logger.info(
            f"Detection completed: camera={request.camera_id}, "
            f"detections={len(detections)}, time={inference_time:.1f}ms"
        )

        return DetectResponse(
            detections=detections,
            inference_time_ms=inference_time,
        )

    except Exception as e:
        logger.error(f"Detection failed: {e}")
        return DetectResponse(
            detections=[],
            inference_time_ms=(time.time() - start_time) * 1000,
            error=str(e),
        )


@app.get("/api/models")
async def list_models():
    """List available YOLO models."""
    return {
        "current": MODEL_NAME,
        "available": [
            "yolov8n.pt",  # Nano - fastest
            "yolov8s.pt",  # Small
            "yolov8m.pt",  # Medium
            "yolov8l.pt",  # Large
            "yolov8x.pt",  # Extra Large - most accurate
            "yolo11n.pt",  # YOLO11 Nano
            "yolo11s.pt",  # YOLO11 Small
            "yolo11m.pt",  # YOLO11 Medium
        ],
    }


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=PORT)
