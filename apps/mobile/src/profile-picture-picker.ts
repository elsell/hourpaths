import { cacheDirectory, deleteAsync } from 'expo-file-system/legacy';
import { launchImageLibraryAsync } from 'expo-image-picker';
import { PictureFailure, validatePictureBytes } from '@hourpaths/client-core';

/** System UI owns selection/cropping; no broad library permission or camera access. */
export async function pickProfilePicture(): Promise<string | null> {
  const result = await launchImageLibraryAsync({
    mediaTypes: ['images'], allowsEditing: true, aspect: [1, 1],
    base64: true, exif: false, quality: 1,
  });
  if (result.canceled) return null;
  const asset = result.assets[0];
  try {
    if (!asset?.base64) throw new PictureFailure('invalid');
    return validatePictureBytes(asset.base64);
  } finally {
    // Only this picker result inside our cache; never remove a library asset.
    if (asset?.uri && cacheDirectory && asset.uri.startsWith(cacheDirectory)) {
      await deleteAsync(asset.uri, { idempotent: true });
    }
  }
}
