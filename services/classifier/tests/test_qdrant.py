"""Unit tests for app.qdrant -- the mongo_id -> Qdrant point ID derivation."""

import uuid

from app.qdrant import mongo_id_to_point_id


def test_mongo_id_to_point_id_is_valid_uuid():
    point_id = mongo_id_to_point_id("507f1f77bcf86cd799439011")
    # Must not raise -- confirms it's a valid UUID string.
    parsed = uuid.UUID(point_id)
    assert str(parsed) == point_id


def test_mongo_id_to_point_id_is_deterministic():
    mongo_id = "507f1f77bcf86cd799439011"
    assert mongo_id_to_point_id(mongo_id) == mongo_id_to_point_id(mongo_id)


def test_mongo_id_to_point_id_different_inputs_different_outputs():
    id_a = mongo_id_to_point_id("507f1f77bcf86cd799439011")
    id_b = mongo_id_to_point_id("507f1f77bcf86cd799439012")
    assert id_a != id_b
