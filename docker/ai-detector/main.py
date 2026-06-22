import os
import time
import json
import base64
import logging
import threading
from typing import List, Optional
from contextlib import asynccontextmanager

import requests
import cv2
import numpy as np
from ultralytics import YOLO

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("gocv-yolo")

# Configuration
NVR_URL = os.getenv("NVR_URL", "http://host.docker.internal:9090")
NVR_AUTH = os.getenv("NVR_AUTH", "")
YOLO_MODEL = os.getenv("YOLO_MODEL", "yolov8n.pt")
CONFIDENCE = float(os.getenv("CONFIDENCE", "0.5"))
NMS_THRESHOLD = float(os.getenv("NMS_THRESHOLD", "0.4"))
FRAME_SKIP = int(os.getenv("FRAME_SKIP", "5"))
SYNC_INTERVAL = int(os.getenv("SYNC_INTERVAL", "30"))
RTSP_PORT = int(os.getenv("RTSP_PORT", "15544"))

# Global state
model = None
active_cameras = {}

def get_auth_headers():
    headers = {"Content-Type": "application/json"}
    if NVR_AUTH:
        headers["Authorization"] = f"Basic {NVR_AUTH}"
    return headers

def fetch_cameras():
    """Fetch camera list from lalmax-nvr API"""
    try:
        resp = requests.get(
            f"{NVR_URL}/api/cameras",
            headers=get_auth_headers(),
            timeout=10
        )
        if resp.status_code == 200:
            return resp.json()
        else:
            logger.error(f"Failed to fetch cameras: HTTP {resp.status_code}")
            return []
    except Exception as e:
        logger.error(f"Failed to fetch cameras: {e}")
        return []

def get_rtsp_url(camera):
    """Get RTSP URL for a camera"""
    url = camera.get("url", "")
    camera_id = camera.get("id", "")
    
    # If URL is already RTSP, use it directly
    if url.startswith("rtsp://"):
        return url
    
    # For ONVIF cameras, construct RTSP URL from NVR's lalmax stream
    nvr_host = NVR_URL.replace("http://", "").replace("https://", "").split(":")[0]
    return f"rtsp://{nvr_host}:{RTSP_PORT}/live/{camera_id}"

def send_webhook(camera_id, detections, frame_num):
    """Send detection results to lalmax-nvr webhook"""
    payload = {
        "camera_id": camera_id,
        "pts": frame_num,
        "timestamp": int(time.time() * 1000),
        "detections": detections
    }
    
    try:
        resp = requests.post(
            f"{NVR_URL}/api/ai/webhook",
            json=payload,
            headers=get_auth_headers(),
            timeout=5
        )
        if resp.status_code != 200:
            logger.warning(f"Webhook returned status {resp.status_code}")
    except Exception as e:
        logger.error(f"Failed to send webhook: {e}")

def process_stream(camera_id, camera_name, stream_url):
    """Process a single camera stream"""
    logger.info(f"[{camera_id}] Starting AI detection for: {camera_name} ({stream_url})")
    
    while True:
        try:
            cap = cv2.VideoCapture(stream_url)
            if not cap.isOpened():
                logger.error(f"[{camera_id}] Failed to open stream, retrying in 5s...")
                time.sleep(5)
                continue
            
            logger.info(f"[{camera_id}] Connected to stream")
            frame_count = 0
            
            while True:
                ret, frame = cap.read()
                if not ret:
                    logger.warning(f"[{camera_id}] Stream ended, reconnecting...")
                    break
                
                frame_count += 1
                if frame_count % FRAME_SKIP != 0:
                    continue
                
                # Run YOLO detection
                results = model(frame, conf=CONFIDENCE, iou=NMS_THRESHOLD, verbose=False)
                
                detections = []
                for result in results:
                    boxes = result.boxes
                    if boxes is not None:
                        for box in boxes:
                            x1, y1, x2, y2 = box.xyxy[0].tolist()
                            h, w = frame.shape[:2]
                            
                            # Normalize coordinates
                            detection = {
                                "label": model.names[int(box.cls[0])],
                                "confidence": float(box.conf[0]),
                                "box": [
                                    x1 / w,  # x
                                    y1 / h,  # y
                                    (x2 - x1) / w,  # width
                                    (y2 - y1) / h   # height
                                ]
                            }
                            detections.append(detection)
                
                if detections:
                    logger.info(f"[{camera_id}] Detected {len(detections)} objects")
                    send_webhook(camera_id, detections, frame_count)
            
            cap.release()
            time.sleep(2)
            
        except Exception as e:
            logger.error(f"[{camera_id}] Error: {e}")
            time.sleep(5)

def sync_cameras():
    """Sync camera list from NVR and start processing for new cameras"""
    global active_cameras
    
    cameras = fetch_cameras()
    
    for cam in cameras:
        cam_id = cam.get("id", "")
        if not cam_id:
            continue
        
        if not cam.get("enabled", False):
            continue
        
        protocol = cam.get("protocol", "")
        if protocol not in ("onvif", "rtsp"):
            continue
        
        if cam_id in active_cameras:
            continue
        
        rtsp_url = get_rtsp_url(cam)
        if not rtsp_url:
            logger.warning(f"No RTSP URL for camera {cam_id}, skipping")
            continue
        
        active_cameras[cam_id] = True
        
        # Start processing in a separate thread
        thread = threading.Thread(
            target=process_stream,
            args=(cam_id, cam.get("name", "Unknown"), rtsp_url),
            daemon=True
        )
        thread.start()
        logger.info(f"Started processing for camera: {cam_id}")

def camera_sync_loop():
    """Periodically sync camera list"""
    while True:
        try:
            sync_cameras()
        except Exception as e:
            logger.error(f"Camera sync error: {e}")
        time.sleep(SYNC_INTERVAL)

def health_check():
    """Simple health check endpoint"""
    from http.server import HTTPServer, BaseHTTPRequestHandler
    
    class HealthHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            if self.path == "/health":
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                response = {
                    "status": "healthy",
                    "model": YOLO_MODEL,
                    "active_cameras": len(active_cameras)
                }
                self.wfile.write(json.dumps(response).encode())
            else:
                self.send_response(404)
                self.end_headers()
        
        def log_message(self, format, *args):
            pass  # Suppress health check logs
    
    port = int(os.getenv("PORT", "8080"))
    server = HTTPServer(("0.0.0.0", port), HealthHandler)
    logger.info(f"Health server listening on :{port}")
    server.serve_forever()

def main():
    global model
    
    logger.info(f"NVR URL: {NVR_URL}")
    logger.info(f"YOLO Model: {YOLO_MODEL}")
    logger.info(f"Confidence: {CONFIDENCE}")
    logger.info(f"Frame Skip: {FRAME_SKIP}")
    
    # Load YOLO model
    logger.info(f"Loading YOLO model: {YOLO_MODEL}")
    model = YOLO(YOLO_MODEL)
    logger.info("YOLO model loaded successfully")
    
    # Start health check server
    health_thread = threading.Thread(target=health_check, daemon=True)
    health_thread.start()
    
    # Start camera sync loop
    sync_thread = threading.Thread(target=camera_sync_loop, daemon=True)
    sync_thread.start()
    
    # Initial sync
    sync_cameras()
    
    # Keep running
    logger.info("Service started, waiting for cameras...")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        logger.info("Shutting down...")

if __name__ == "__main__":
    main()
