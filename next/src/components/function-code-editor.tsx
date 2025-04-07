"use client";

import { useState } from "react";

export function FunctionCodeEditor() {
  const [code, setCode] = useState(`exports.handler = async (event) => {
  console.log('Processing event:', event);
  
  try {
    // Get the image from the event
    const image = event.image || event.body?.image;
    
    if (!image) {
      throw new Error('No image provided in the event');
    }
    
    // Process the image (this is a placeholder)
    console.log('Processing image...');
    
    // In a real implementation, you would:
    // 1. Download the image from a URL or decode from base64
    // 2. Use a library like Sharp to resize/optimize the image
    // 3. Upload the processed image to a storage service
    
    const result = {
      success: true,
      message: 'Image processed successfully',
      processedImageUrl: 'https://storage.example.com/processed-images/image-123.jpg'
    };
    
    return {
      statusCode: 200,
      body: JSON.stringify(result)
    };
  } catch (error) {
    console.error('Error processing image:', error);
    
    return {
      statusCode: 500,
      body: JSON.stringify({
        success: false,
        message: 'Error processing image',
        error: error.message
      })
    };
  }
};`);

  return (
    <div className="rounded-md border">
      <div className="flex items-center justify-between border-b bg-muted/50 px-3 py-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">index.js</span>
        </div>
      </div>
      <div className="relative min-h-[400px] w-full overflow-auto">
        <pre className="p-4 text-sm">
          <code className="language-javascript">{code}</code>
        </pre>
      </div>
    </div>
  );
}
