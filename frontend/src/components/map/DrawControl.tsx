import '@geoman-io/leaflet-geoman-free'
import type { LatLng as LeafletLatLng, Polygon } from 'leaflet'
import { useEffect, useRef } from 'react'
import { useMap } from 'react-leaflet'
import type { LatLng } from '../../api/types'
import { toClosedRing } from '../../lib/geometry'

/** Adds geoman's polygon/rectangle tools while mounted and reports each finished shape. */
export function DrawControl({ onDraw }: { onDraw: (ring: LatLng[]) => void }) {
  const map = useMap()
  const onDrawRef = useRef(onDraw)
  useEffect(() => {
    onDrawRef.current = onDraw
  })

  useEffect(() => {
    map.pm.addControls({
      position: 'topleft',
      drawPolygon: true,
      drawRectangle: true,
      drawMarker: false,
      drawCircleMarker: false,
      drawPolyline: false,
      drawCircle: false,
      drawText: false,
      editMode: false,
      dragMode: false,
      cutPolygon: false,
      removalMode: false,
      rotateMode: false,
    })

    const handleCreate = ({ layer }: { layer: unknown }) => {
      const outerRing = (layer as Polygon).getLatLngs()[0] as LeafletLatLng[]
      // The drawn layer is removed; the page shows the shape as a draft from its own state, so
      // what is on the map always matches what will be submitted.
      map.removeLayer(layer as Polygon)
      onDrawRef.current(toClosedRing(outerRing))
    }

    map.on('pm:create', handleCreate)
    return () => {
      map.off('pm:create', handleCreate)
      map.pm.disableDraw()
      map.pm.removeControls()
    }
  }, [map])

  return null
}
